package api

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"
	"time"
)

// Some presets for different amounts of information that can be requested for fields;
// it is recommended to only request the information that is actually needed.
var (
	HiDriveObjectNoMetadataFields            = []string{"name", "type"}
	HiDriveObjectWithMetadataFields          = append(HiDriveObjectNoMetadataFields, "id", "size", "mtime", "chash")
	HiDriveObjectWithDirectoryMetadataFields = append(HiDriveObjectWithMetadataFields, "nmembers")
	DirectoryContentFields                   = []string{"nmembers"}
)

// QueryParameters represents the parameters passed to an API-call.
type QueryParameters struct {
	url.Values
}

// NewQueryParameters initializes an instance of QueryParameters and
// returns a pointer to it.
func NewQueryParameters() *QueryParameters {
	return &QueryParameters{url.Values{}}
}

// SetFileInDirectory sets the appropriate parameters
// to specify a path to a file in a directory.
// This is used by requests that work with paths for files that do not exist yet.
// (For example when creating a file).
// Most requests use the format produced by SetPath(...).
func (p *QueryParameters) SetFileInDirectory(filePath string) {
	directory, file := path.Split(path.Clean(filePath))
	p.Set("dir", path.Clean(directory))
	p.Set("name", file)
	// NOTE: It would be possible to switch to pid-based requests
	// by modifying this function.
}

// SetPath sets the appropriate parameters to access the given path.
func (p *QueryParameters) SetPath(objectPath string) {
	p.Set("path", path.Clean(objectPath))
	// NOTE: It would be possible to switch to pid-based requests
	// by modifying this function.
}

// SetTime sets the key to the time-value. It replaces any existing values.
func (p *QueryParameters) SetTime(key string, value time.Time) error {
	valueAPI := Time(value)
	valueBytes, err := json.Marshal(&valueAPI)
	if err != nil {
		return err
	}
	p.Set(key, string(valueBytes))
	return nil
}

// AddList adds the given values as a list
// with each value separated by the separator.
// It appends to any existing values associated with key.
func (p *QueryParameters) AddList(key string, separator string, values ...string) {
	original := p.Get(key)
	p.Set(key, strings.Join(values, separator))
	if original != "" {
		p.Set(key, original+separator+p.Get(key))
	}
}

// AddFields sets the appropriate parameter to access the given fields.
// The given fields will be appended to any other existing fields.
func (p *QueryParameters) AddFields(prefix string, fields ...string) {
	modifiedFields := make([]string, len(fields))
	for i, field := range fields {
		modifiedFields[i] = prefix + field
	}
	p.AddList("fields", ",", modifiedFields...)
}
