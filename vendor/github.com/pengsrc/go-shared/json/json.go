package json

import (
	"bytes"
	"encoding/json"
)

// Encode encode given interface to json byte slice.
func Encode(source interface{}, unescape bool) ([]byte, error) {
	bytesResult, err := json.Marshal(source)
	if err != nil {
		return []byte{}, err
	}

	if unescape {
		bytesResult = bytes.Replace(bytesResult, []byte("\\u003c"), []byte("<"), -1)
		bytesResult = bytes.Replace(bytesResult, []byte("\\u003e"), []byte(">"), -1)
		bytesResult = bytes.Replace(bytesResult, []byte("\\u0026"), []byte("&"), -1)
	}

	return bytesResult, nil
}

// Decode decode given json byte slice to corresponding struct.
func Decode(content []byte, destinations ...interface{}) (interface{}, error) {
	var destination interface{}
	var err error
	if len(destinations) == 1 {
		destination = destinations[0]
		err = json.Unmarshal(content, destination)
	} else {
		err = json.Unmarshal(content, &destination)
	}

	if err != nil {
		return nil, err
	}
	return destination, err
}

// FormatToReadable formats given json byte slice prettily.
func FormatToReadable(source []byte) ([]byte, error) {
	var out bytes.Buffer
	err := json.Indent(&out, source, "", "  ") // Using 2 space indent
	if err != nil {
		return []byte{}, err
	}

	return out.Bytes(), nil
}
