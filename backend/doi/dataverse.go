// Implementation for Dataverse

package doi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// Returns true if resolvedURL is likely a DOI hosted on a Dataverse intallation
func activateDataverse(resolvedURL *url.URL) (isActive bool) {
	queryValues := resolvedURL.Query()
	persistentID := queryValues.Get("persistentId")
	return persistentID != ""
}

// Resolve the main API endpoint for a DOI hosted on a Dataverse installation
func resolveDataverseEndpoint(resolvedURL *url.URL) (provider Provider, endpoint *url.URL, err error) {
	queryValues := resolvedURL.Query()
	persistentID := queryValues.Get("persistentId")

	query := url.Values{}
	query.Add("persistentId", persistentID)
	endpointURL := resolvedURL.ResolveReference(&url.URL{Path: "/api/datasets/:persistentId/", RawQuery: query.Encode()})

	return Dataverse, endpointURL, nil
}

// dataverseProvider implements the doiProvider interface for Dataverse installations
type dataverseProvider struct {
	f *Fs
}

// ListEntries returns the full list of entries found at the remote, regardless of root
func (dp *dataverseProvider) ListEntries(ctx context.Context) (entries []*Object, err error) {
	// Use the cache if populated
	cachedEntries, found := dp.f.cache.GetMaybe("files")
	if found {
		parsedEntries, ok := cachedEntries.([]Object)
		if ok {
			for _, entry := range parsedEntries {
				newEntry := entry
				entries = append(entries, &newEntry)
			}
			return entries, nil
		}
	}

	filesURL := dp.f.endpoint
	var res *http.Response
	var result api.DataverseDatasetResponse
	opts := rest.Opts{
		Method:     "GET",
		Path:       strings.TrimLeft(filesURL.EscapedPath(), "/"),
		Parameters: filesURL.Query(),
	}
	err = dp.f.pacer.Call(func() (bool, error) {
		res, err = dp.f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, fmt.Errorf("readDir failed: %w", err)
	}
	modTime, modTimeErr := time.Parse(time.RFC3339, result.Data.LatestVersion.LastUpdateTime)
	if modTimeErr != nil {
		fs.Logf(dp.f, "error: could not parse last update time %v", modTimeErr)
		modTime = timeUnset
	}
	for _, file := range result.Data.LatestVersion.Files {
		contentURLPath := fmt.Sprintf("/api/access/datafile/%d", file.DataFile.ID)
		query := url.Values{}
		query.Add("format", "original")
		contentURL := dp.f.endpoint.ResolveReference(&url.URL{Path: contentURLPath, RawQuery: query.Encode()})
		entry := &Object{
			fs:          dp.f,
			remote:      path.Join(file.DirectoryLabel, file.DataFile.Filename),
			contentURL:  contentURL.String(),
			size:        file.DataFile.FileSize,
			modTime:     modTime,
			md5:         file.DataFile.MD5,
			contentType: file.DataFile.ContentType,
		}
		if file.DataFile.OriginalFileName != "" {
			entry.remote = path.Join(file.DirectoryLabel, file.DataFile.OriginalFileName)
			entry.size = file.DataFile.OriginalFileSize
			entry.contentType = file.DataFile.OriginalFileFormat
		}
		entries = append(entries, entry)
	}
	// Populate the cache
	cacheEntries := []Object{}
	for _, entry := range entries {
		cacheEntries = append(cacheEntries, *entry)
	}
	dp.f.cache.Put("files", cacheEntries)
	return entries, nil
}

func newDataverseProvider(f *Fs) doiProvider {
	return &dataverseProvider{
		f: f,
	}
}
