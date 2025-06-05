// Implementation for Zenodo

package doi

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

var zenodoRecordRegex = regexp.MustCompile(`zenodo[.](.+)`)

// Resolve the main API endpoint for a DOI hosted on Zenodo
func resolveZenodoEndpoint(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, resolvedURL *url.URL, doi string) (provider Provider, endpoint *url.URL, err error) {
	match := zenodoRecordRegex.FindStringSubmatch(doi)
	if match == nil {
		return "", nil, fmt.Errorf("could not derive API endpoint URL from '%s'", resolvedURL.String())
	}

	recordID := match[1]
	endpointURL := resolvedURL.ResolveReference(&url.URL{Path: "/api/records/" + recordID})

	var result api.InvenioRecordResponse
	opts := rest.Opts{
		Method:  "GET",
		RootURL: endpointURL.String(),
	}
	err = pacer.Call(func() (bool, error) {
		res, err := srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return "", nil, err
	}

	endpointURL, err = url.Parse(result.Links.Self)
	if err != nil {
		return "", nil, err
	}

	return Zenodo, endpointURL, nil
}
