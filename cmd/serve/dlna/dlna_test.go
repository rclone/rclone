package dlna

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/rclone/rclone/vfs"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/dlna/dlnaflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dlnaServer *server
	testURL    string
)

const (
	testBindAddress = "localhost:0"
)

func startServer(t *testing.T, f fs.Fs) {
	opt := dlnaflags.DefaultOpt
	opt.ListenAddr = testBindAddress
	dlnaServer = newServer(f, &opt)
	assert.NoError(t, dlnaServer.Serve())
	testURL = "http://" + dlnaServer.HTTPConn.Addr().String() + "/"
}

func TestInit(t *testing.T) {
	config.LoadConfig()

	f, err := fs.NewFs("testdata/files")
	l, _ := f.List(context.Background(), "")
	fmt.Println(l)
	require.NoError(t, err)

	startServer(t, f)
}

// Make sure that it serves rootDesc.xml (SCPD in uPnP parlance).
func TestRootSCPD(t *testing.T) {
	req, err := http.NewRequest("GET", testURL+"rootDesc.xml", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	// Make sure that the SCPD contains a CDS service.
	require.Contains(t, string(body),
		"<serviceType>urn:schemas-upnp-org:service:ContentDirectory:1</serviceType>")
	// Make sure that the SCPD contains a CM service.
	require.Contains(t, string(body),
		"<serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType>")
	// Ensure that the SCPD url is configured.
	require.Regexp(t, "<SCPDURL>/.*</SCPDURL>", string(body))
}

// Make sure that it serves content from the remote.
func TestServeContent(t *testing.T) {
	itemPath := "/small_jpeg.jpg"
	pathQuery := url.QueryEscape(itemPath)
	req, err := http.NewRequest("GET", testURL+"res?path="+pathQuery, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer fs.CheckClose(resp.Body, &err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	actualContents, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)

	// Now compare the contents with the golden file.
	node, err := dlnaServer.vfs.Stat(itemPath)
	assert.NoError(t, err)
	goldenFile := node.(*vfs.File)
	goldenReader, err := goldenFile.Open(os.O_RDONLY)
	assert.NoError(t, err)
	defer fs.CheckClose(goldenReader, &err)
	goldenContents, err := ioutil.ReadAll(goldenReader)
	assert.NoError(t, err)

	require.Equal(t, goldenContents, actualContents)
}
