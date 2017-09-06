package yaml

import (
	"gopkg.in/yaml.v2"
)

// Encode encode given interface to yaml byte slice.
func Encode(source interface{}) ([]byte, error) {
	bytesResult, err := yaml.Marshal(source)
	if err != nil {
		return []byte{}, err
	}

	return bytesResult, nil
}

// Decode decode given yaml byte slice to corresponding struct.
func Decode(content []byte, destinations ...interface{}) (interface{}, error) {
	var destination interface{}
	var err error
	if len(destinations) == 1 {
		destination = destinations[0]
		err = yaml.Unmarshal(content, destination)
	} else {
		err = yaml.Unmarshal(content, &destination)
	}

	if err != nil {
		return nil, err
	}
	return destination, err
}
