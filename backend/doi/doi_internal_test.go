package doi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var remoteName = "TestDoi"

func TestParseDoi(t *testing.T) {
	// 10.1000/182 -> 10.1000/182
	doi := "10.1000/182"
	parsed := parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// https://doi.org/10.1000/182 -> 10.1000/182
	doi = "https://doi.org/10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// https://dx.doi.org/10.1000/182 -> 10.1000/182
	doi = "https://dxdoi.org/10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// doi:10.1000/182 -> 10.1000/182
	doi = "doi:10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// doi://10.1000/182 -> 10.1000/182
	doi = "doi://10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)
}

// prepareMockDoiResolverServer prepares a test server to resolve DOIs
func prepareMockDoiResolverServer(t *testing.T, resolvedURL string) (doiResolverAPIURL string) {
	mux := http.NewServeMux()

	// Handle requests for resolving DOIs
	mux.HandleFunc("GET /api/handles/{handle...}", func(w http.ResponseWriter, r *http.Request) {
		// Check that we are resolving a DOI
		handle := strings.TrimPrefix(r.URL.Path, "/api/handles/")
		assert.NotEmpty(t, handle)
		index := r.URL.Query().Get("index")
		assert.Equal(t, "1", index)

		// Return the most basic response
		result := api.DoiResolverResponse{
			ResponseCode: 1,
			Handle:       handle,
			Values: []api.DoiResolverResponseValue{
				{
					Index: 1,
					Type:  "URL",
					Data: api.DoiResolverResponseValueData{
						Format: "string",
						Value:  resolvedURL,
					},
				},
			},
		}
		resultBytes, err := json.Marshal(result)
		require.NoError(t, err)
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(resultBytes)
		require.NoError(t, err)
	})

	// Make the test server
	ts := httptest.NewServer(mux)

	// Close the server at the end of the test
	t.Cleanup(ts.Close)

	return ts.URL + "/api"
}

func md5Sum(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// prepareMockZenodoServer prepares a test server that mocks Zenodo.org
func prepareMockZenodoServer(t *testing.T, files map[string]string) *httptest.Server {
	mux := http.NewServeMux()

	// Handle requests for a single record
	mux.HandleFunc("GET /api/records/{recordID...}", func(w http.ResponseWriter, r *http.Request) {
		// Check that we are returning data about a single record
		recordID := strings.TrimPrefix(r.URL.Path, "/api/records/")
		assert.NotEmpty(t, recordID)

		// Return the most basic response
		selfURL, err := url.Parse("http://" + r.Host)
		require.NoError(t, err)
		selfURL = selfURL.JoinPath(r.URL.String())
		result := api.InvenioRecordResponse{
			Links: api.InvenioRecordResponseLinks{
				Self: selfURL.String(),
			},
		}
		resultBytes, err := json.Marshal(result)
		require.NoError(t, err)
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(resultBytes)
		require.NoError(t, err)
	})
	// Handle requests for listing files in a record
	mux.HandleFunc("GET /api/records/{record}/files", func(w http.ResponseWriter, r *http.Request) {
		// Return the most basic response
		filesBaseURL, err := url.Parse("http://" + r.Host)
		require.NoError(t, err)
		filesBaseURL = filesBaseURL.JoinPath("/api/files/")

		entries := []api.InvenioFilesResponseEntry{}
		for filename, contents := range files {
			entries = append(entries,
				api.InvenioFilesResponseEntry{
					Key:      filename,
					Checksum: md5Sum(contents),
					Size:     int64(len(contents)),
					Updated:  time.Now().UTC().Format(time.RFC3339),
					MimeType: "text/plain; charset=utf-8",
					Links: api.InvenioFilesResponseEntryLinks{
						Content: filesBaseURL.JoinPath(filename).String(),
					},
				},
			)
		}

		result := api.InvenioFilesResponse{
			Entries: entries,
		}
		resultBytes, err := json.Marshal(result)
		require.NoError(t, err)
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(resultBytes)
		require.NoError(t, err)
	})
	// Handle requests for file contents
	mux.HandleFunc("/api/files/{file}", func(w http.ResponseWriter, r *http.Request) {
		// Check that we are returning the contents of a file
		filename := strings.TrimPrefix(r.URL.Path, "/api/files/")
		assert.NotEmpty(t, filename)
		contents, found := files[filename]
		if !found {
			w.WriteHeader(404)
			return
		}

		// Return the most basic response
		_, err := w.Write([]byte(contents))
		require.NoError(t, err)
	})

	// Make the test server
	ts := httptest.NewServer(mux)

	// Close the server at the end of the test
	t.Cleanup(ts.Close)

	return ts
}

func TestZenodoRemote(t *testing.T) {
	recordID := "2600782"
	doi := "10.5281/zenodo.2600782"

	// The files in the dataset
	files := map[string]string{
		"README.md": "This is a dataset.",
		"data.txt":  "Some data",
	}

	ts := prepareMockZenodoServer(t, files)
	resolvedURL := ts.URL + "/record/" + recordID

	doiResolverAPIURL := prepareMockDoiResolverServer(t, resolvedURL)

	testConfig := configmap.Simple{
		"type":                 "doi",
		"doi":                  doi,
		"provider":             "zenodo",
		"doi_resolver_api_url": doiResolverAPIURL,
	}
	f, err := NewFs(context.Background(), remoteName, "", testConfig)
	require.NoError(t, err)

	// Test listing the DOI files
	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)

	sort.Sort(entries)

	require.Equal(t, len(files), len(entries))

	e := entries[0]
	assert.Equal(t, "README.md", e.Remote())
	assert.Equal(t, int64(18), e.Size())
	_, ok := e.(*Object)
	assert.True(t, ok)

	e = entries[1]
	assert.Equal(t, "data.txt", e.Remote())
	assert.Equal(t, int64(9), e.Size())
	_, ok = e.(*Object)
	assert.True(t, ok)

	// Test reading the DOI files
	o, err := f.NewObject(context.Background(), "README.md")
	require.NoError(t, err)
	assert.Equal(t, int64(18), o.Size())
	md5Hash, err := o.Hash(context.Background(), hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "464352b1cab5240e44528a56fda33d9d", md5Hash)
	fd, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	assert.Equal(t, []byte(files["README.md"]), data)
	do, ok := o.(fs.MimeTyper)
	require.True(t, ok)
	assert.Equal(t, "text/plain; charset=utf-8", do.MimeType(context.Background()))

	o, err = f.NewObject(context.Background(), "data.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(9), o.Size())
	md5Hash, err = o.Hash(context.Background(), hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "5b82f8bf4df2bfb0e66ccaa7306fd024", md5Hash)
	fd, err = o.Open(context.Background())
	require.NoError(t, err)
	data, err = io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	assert.Equal(t, []byte(files["data.txt"]), data)
	do, ok = o.(fs.MimeTyper)
	require.True(t, ok)
	assert.Equal(t, "text/plain; charset=utf-8", do.MimeType(context.Background()))
}
