package json

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONDecodeUnknown(t *testing.T) {
	jsonString := `{
		"key1" : "This is a string.",
		"key2" : 10.50,
   		"key3": [null, {"nestedKey1": "Another string"}]
	}`

	anyData, err := Decode([]byte(jsonString))
	assert.NoError(t, err)
	data := anyData.(map[string]interface{})
	assert.Equal(t, 10.50, data["key2"])

	var anotherData interface{}
	_, err = Decode([]byte(jsonString), &anotherData)
	assert.NoError(t, err)
	data = anyData.(map[string]interface{})
	assert.Equal(t, 10.50, data["key2"])

	_, err = Decode([]byte(`- - -`), &JSONMustError{})
	assert.Error(t, err)
}

func TestJSONDecodeKnown(t *testing.T) {
	type SampleJSON struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	sampleJSONString := `{"name": "NAME"}`

	sample := SampleJSON{Name: "NaMe", Description: "DeScRiPtIoN"}
	anyDataPointer, err := Decode([]byte(sampleJSONString), &sample)
	assert.NoError(t, err)
	data := anyDataPointer.(*SampleJSON)
	assert.Equal(t, "NAME", sample.Name)
	assert.Equal(t, "DeScRiPtIoN", sample.Description)
	assert.Equal(t, "NAME", (*data).Name)
	assert.Equal(t, "DeScRiPtIoN", (*data).Description)
}

func TestJSONEncode(t *testing.T) {
	type SampleJSON struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	sample := SampleJSON{Name: "NaMe", Description: "DeScRiPtIoN"}

	jsonBytes, err := Encode(sample, true)
	assert.NoError(t, err)
	assert.Equal(t, `{"name":"NaMe","description":"DeScRiPtIoN"}`, string(jsonBytes))

	_, err = Encode(&JSONMustError{}, true)
	assert.Error(t, err)
}

func TestJSONFormatToReadable(t *testing.T) {
	sampleJSONString := `{"name": "NAME"}`

	jsonBytes, err := FormatToReadable([]byte(sampleJSONString))
	assert.NoError(t, err)
	assert.Equal(t, "{\n  \"name\": \"NAME\"\n}", string(jsonBytes))

	_, err = FormatToReadable([]byte(`XXXXX`))
	assert.Error(t, err)
}

type JSONMustError struct{}

func (*JSONMustError) MarshalJSON() ([]byte, error) {
	return []byte{}, errors.New("marshal error")
}
