// Implementation for InvenioRDM

package doi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

var invenioRecordRegex = regexp.MustCompile(`\/records?\/(.+)`)

// Returns true if resolvedURL is likely a DOI hosted on an InvenioRDM intallation
func activateInvenio(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, resolvedURL *url.URL) (isActive bool) {
	_, _, err := resolveInvenioEndpoint(ctx, srv, pacer, resolvedURL)
	return err == nil
}

// Resolve the main API endpoint for a DOI hosted on an InvenioRDM installation
func resolveInvenioEndpoint(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, resolvedURL *url.URL) (provider Provider, endpoint *url.URL, err error) {
	var res *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: resolvedURL.String(),
	}
	err = pacer.Call(func() (bool, error) {
		res, err = srv.Call(ctx, &opts)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return "", nil, err
	}

	// First, attempt to grab the API URL from the headers
	var linksetURL *url.URL
	links := parseLinkHeader(res.Header.Get("Link"))
	for _, link := range links {
		if link.Rel == "linkset" && link.Type == "application/linkset+json" {
			parsed, err := url.Parse(link.Href)
			if err == nil {
				linksetURL = parsed
				break
			}
		}
	}

	if linksetURL != nil {
		endpoint, err = checkInvenioAPIURL(ctx, srv, pacer, linksetURL)
		if err == nil {
			return Invenio, endpoint, nil
		}
		fs.Logf(nil, "using linkset URL failed: %s", err.Error())
	}

	// If there is no linkset header, try to grab the record ID from the URL
	recordID := ""
	resURL := res.Request.URL
	match := invenioRecordRegex.FindStringSubmatch(resURL.EscapedPath())
	if match != nil {
		recordID = match[1]
		guessedURL := res.Request.URL.ResolveReference(&url.URL{
			Path: "/api/records/" + recordID,
		})
		endpoint, err = checkInvenioAPIURL(ctx, srv, pacer, guessedURL)
		if err == nil {
			return Invenio, endpoint, nil
		}
		fs.Logf(nil, "guessing the URL failed: %s", err.Error())
	}

	return "", nil, fmt.Errorf("could not resolve the Invenio API endpoint for '%s'", resolvedURL.String())
}

func checkInvenioAPIURL(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, resolvedURL *url.URL) (endpoint *url.URL, err error) {
	var result api.InvenioRecordResponse
	opts := rest.Opts{
		Method:  "GET",
		RootURL: resolvedURL.String(),
	}
	err = pacer.Call(func() (bool, error) {
		res, err := srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, err
	}
	if result.Links.Self == "" {
		return nil, fmt.Errorf("could not parse API response from '%s'", resolvedURL.String())
	}
	return url.Parse(result.Links.Self)
}

// invenioProvider implements the doiProvider interface for InvenioRDM installations
type invenioProvider struct {
	f *Fs
}

// ListEntries returns the full list of entries found at the remote, regardless of root
func (ip *invenioProvider) ListEntries(ctx context.Context) (entries []*Object, err error) {
	// Use the cache if populated
	cachedEntries, found := ip.f.cache.GetMaybe("files")
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

	filesURL := ip.f.endpoint.JoinPath("files")
	var result api.InvenioFilesResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   strings.TrimLeft(filesURL.EscapedPath(), "/"),
	}
	err = ip.f.pacer.Call(func() (bool, error) {
		res, err := ip.f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, fmt.Errorf("readDir failed: %w", err)
	}
	for _, file := range result.Entries {
		modTime, modTimeErr := time.Parse(time.RFC3339, file.Updated)
		if modTimeErr != nil {
			fs.Logf(ip.f, "error: could not parse last update time %v", modTimeErr)
			modTime = timeUnset
		}
		entry := &Object{
			fs:          ip.f,
			remote:      file.Key,
			contentURL:  file.Links.Content,
			size:        file.Size,
			modTime:     modTime,
			contentType: file.MimeType,
			md5:         strings.TrimPrefix(file.Checksum, "md5:"),
		}
		entries = append(entries, entry)
	}
	// Populate the cache
	cacheEntries := []Object{}
	for _, entry := range entries {
		cacheEntries = append(cacheEntries, *entry)
	}
	ip.f.cache.Put("files", cacheEntries)
	return entries, nil
}

func newInvenioProvider(f *Fs) doiProvider {
	return &invenioProvider{
		f: f,
	}
}
