package azblob

import (
	"net/url"
	"strings"
)

const (
	snapshot           = "snapshot"
	SnapshotTimeFormat = "2006-01-02T15:04:05.0000000Z07:00"
)

// A BlobURLParts object represents the components that make up an Azure Storage Container/Blob URL. You parse an
// existing URL into its parts by calling NewBlobURLParts(). You construct a URL from parts by calling URL().
// NOTE: Changing any SAS-related field requires computing a new SAS signature.
type BlobURLParts struct {
	Scheme         string // Ex: "https://"
	Host           string // Ex: "account.blob.core.windows.net"
	ContainerName  string // "" if no container
	BlobName       string // "" if no blob
	Snapshot       string // "" if not a snapshot
	SAS            SASQueryParameters
	UnparsedParams string
}

// NewBlobURLParts parses a URL initializing BlobURLParts' fields including any SAS-related & snapshot query parameters. Any other
// query parameters remain in the UnparsedParams field. This method overwrites all fields in the BlobURLParts object.
func NewBlobURLParts(u url.URL) BlobURLParts {
	up := BlobURLParts{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	// Find the container & blob names (if any)
	if u.Path != "" {
		path := u.Path
		if path[0] == '/' {
			path = path[1:] // If path starts with a slash, remove it
		}

		// Find the next slash (if it exists)
		containerEndIndex := strings.Index(path, "/")
		if containerEndIndex == -1 { // Slash not found; path has container name & no blob name
			up.ContainerName = path
		} else {
			up.ContainerName = path[:containerEndIndex] // The container name is the part between the slashes
			up.BlobName = path[containerEndIndex+1:]    // The blob name is after the container slash
		}
	}

	// Convert the query parameters to a case-sensitive map & trim whitespace
	paramsMap := u.Query()

	up.Snapshot = "" // Assume no snapshot
	if snapshotStr, ok := caseInsensitiveValues(paramsMap).Get(snapshot); ok {
		up.Snapshot = snapshotStr[0]
		// If we recognized the query parameter, remove it from the map
		delete(paramsMap, snapshot)
	}
	up.SAS = newSASQueryParameters(paramsMap, true)
	up.UnparsedParams = paramsMap.Encode()
	return up
}

type caseInsensitiveValues url.Values // map[string][]string
func (values caseInsensitiveValues) Get(key string) ([]string, bool) {
	key = strings.ToLower(key)
	for k, v := range values {
		if strings.ToLower(k) == key {
			return v, true
		}
	}
	return []string{}, false
}

// URL returns a URL object whose fields are initialized from the BlobURLParts fields. The URL's RawQuery
// field contains the SAS, snapshot, and unparsed query parameters.
func (up BlobURLParts) URL() url.URL {
	path := ""
	// Concatenate container & blob names (if they exist)
	if up.ContainerName != "" {
		path += "/" + up.ContainerName
		if up.BlobName != "" {
			path += "/" + up.BlobName
		}
	}

	rawQuery := up.UnparsedParams

	// Concatenate blob snapshot query parameter (if it exists)
	if up.Snapshot != "" {
		if len(rawQuery) > 0 {
			rawQuery += "&"
		}
		rawQuery += snapshot + "=" + up.Snapshot
	}
	sas := up.SAS.Encode()
	if sas != "" {
		if len(rawQuery) > 0 {
			rawQuery += "&"
		}
		rawQuery += sas
	}
	u := url.URL{
		Scheme:   up.Scheme,
		Host:     up.Host,
		Path:     path,
		RawQuery: rawQuery,
	}
	return u
}
