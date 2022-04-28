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
	// file server
	fileServer := http.FileServer(http.Dir(""))

	// test the headers are there then pass on to fileServer
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		what := fmt.Sprintf("%s %s: Header ", r.Method, r.URL.Path)
		assert.Equal(t, headers[1], r.Header.Get(headers[0]), what+headers[0])
		assert.Equal(t, headers[3], r.Header.Get(headers[2]), what+headers[2])
		fileServer.ServeHTTP(w, r)
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

	// any request will do
	_, err := f.Features().About(context.Background())
	require.NoError(t, err)
}
