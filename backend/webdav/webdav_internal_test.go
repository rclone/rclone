package webdav_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/webdav"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	remoteName = "TestWebDAV"
	headers    = []string{"X-Potato", "sausage", "X-Rhubarb", "cucumber"}
)

// prepareServer the test server and return a function to tidy it up afterwards
// with each request the headers option tests are executed
func prepareServer(t *testing.T) (configmap.Simple, func()) {
	// test the headers are there send send a dummy response to About
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		what := fmt.Sprintf("%s %s: Header ", r.Method, r.URL.Path)
		assert.Equal(t, headers[1], r.Header.Get(headers[0]), what+headers[0])
		assert.Equal(t, headers[3], r.Header.Get(headers[2]), what+headers[2])
		_, err := fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns" xmlns:nc="http://nextcloud.org/ns">
<d:response>
 <d:href>/remote.php/webdav/</d:href>
 <d:propstat>
  <d:prop>
   <d:quota-available-bytes>-3</d:quota-available-bytes>
   <d:quota-used-bytes>376461895</d:quota-used-bytes>
  </d:prop>
  <d:status>HTTP/1.1 200 OK</d:status>
 </d:propstat>
</d:response>
</d:multistatus>`)
		require.NoError(t, err)
	})
	// Make the test server
	ts := httptest.NewServer(handler)

	// Configure the remote
	configfile.Install()

	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		// add headers to test the headers option
		"headers": strings.Join(headers, ","),
	}

	// return a function to tidy up
	return m, ts.Close
}

// prepare the test server and return a function to tidy it up afterwards
func prepare(t *testing.T) (fs.Fs, func()) {
	m, tidy := prepareServer(t)

	// Instantiate the WebDAV server
	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	return f, tidy
}

// TestHeaders any request will test the headers option
func TestHeaders(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	// send an About response since that is all the dummy server can return
	_, err := f.Features().About(context.Background())
	require.NoError(t, err)
}
