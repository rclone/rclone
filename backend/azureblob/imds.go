// +build !plan9,!solaris,!js,go1.14

package azureblob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
)

const (
	azureResource      = "https://storage.azure.com"
	imdsAPIVersion     = "2018-02-01"
	msiEndpointDefault = "http://169.254.169.254/metadata/identity/oauth2/token"
)

// This custom type is used to add the port the test server has bound to
// to the request context.
type testPortKey string

type msiIdentifierType int

const (
	msiClientID msiIdentifierType = iota
	msiObjectID
	msiResourceID
)

type userMSI struct {
	Type  msiIdentifierType
	Value string
}

type httpError struct {
	Response *http.Response
}

func (e httpError) Error() string {
	return fmt.Sprintf("HTTP error %v (%v)", e.Response.StatusCode, e.Response.Status)
}

// GetMSIToken attempts to obtain an MSI token from the Azure Instance
// Metadata Service.
func GetMSIToken(ctx context.Context, identity *userMSI) (adal.Token, error) {
	// Attempt to get an MSI token; silently continue if unsuccessful.
	// This code has been lovingly stolen from azcopy's OAuthTokenManager.
	result := adal.Token{}
	req, err := http.NewRequestWithContext(ctx, "GET", msiEndpointDefault, nil)
	if err != nil {
		fs.Debugf(nil, "Failed to create request: %v", err)
		return result, err
	}
	params := req.URL.Query()
	params.Set("resource", azureResource)
	params.Set("api-version", imdsAPIVersion)

	// Specify user-assigned identity if requested.
	if identity != nil {
		switch identity.Type {
		case msiClientID:
			params.Set("client_id", identity.Value)
		case msiObjectID:
			params.Set("object_id", identity.Value)
		case msiResourceID:
			params.Set("mi_res_id", identity.Value)
		default:
			// If this happens, the calling function and this one don't agree on
			// what valid ID types exist.
			return result, fmt.Errorf("unknown MSI identity type specified")
		}
	}
	req.URL.RawQuery = params.Encode()

	// The Metadata header is required by all calls to IMDS.
	req.Header.Set("Metadata", "true")

	// If this function is run in a test, query the test server instead of IMDS.
	testPort, isTest := ctx.Value(testPortKey("testPort")).(int)
	if isTest {
		req.URL.Host = fmt.Sprintf("localhost:%d", testPort)
		req.Host = req.URL.Host
	}

	// Send request
	httpClient := fshttp.NewClient(ctx)
	resp, err := httpClient.Do(req)
	if err != nil {
		return result, errors.Wrap(err, "MSI is not enabled on this VM")
	}
	defer func() { // resp and Body should not be nil
		_, err = io.Copy(ioutil.Discard, resp.Body)
		if err != nil {
			fs.Debugf(nil, "Unable to drain IMDS response: %v", err)
		}
		err = resp.Body.Close()
		if err != nil {
			fs.Debugf(nil, "Unable to close IMDS response: %v", err)
		}
	}()
	// Check if the status code indicates success
	// The request returns 200 currently, add 201 and 202 as well for possible extension.
	switch resp.StatusCode {
	case 200, 201, 202:
		break
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		fs.Errorf(nil, "Couldn't obtain OAuth token from IMDS; server returned status code %d and body: %v", resp.StatusCode, string(body))
		return result, httpError{Response: resp}
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, errors.Wrap(err, "Couldn't read IMDS response")
	}
	// Remove BOM, if any. azcopy does this so I'm following along.
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))

	// This would be a good place to persist the token if a large number of rclone
	// invocations are being made in a short amount of time. If the token is
	// persisted, the azureblob code will need to check for expiry before every
	// storage API call.
	err = json.Unmarshal(b, &result)
	if err != nil {
		return result, errors.Wrap(err, "Couldn't unmarshal IMDS response")
	}

	return result, nil
}
