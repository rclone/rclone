//go:build go1.21

package dlna

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/anacrolix/dms/soap"

	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/vfs"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/dlna/dlnaflags"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dlnaServer *server
	baseURL    string
)

const (
	testBindAddress = "localhost:0"
)

func startServer(t *testing.T, f fs.Fs) {
	opt := dlnaflags.DefaultOpt
	opt.ListenAddr = testBindAddress
	var err error
	dlnaServer, err = newServer(f, &opt)
	assert.NoError(t, err)
	assert.NoError(t, dlnaServer.Serve())
	baseURL = "http://" + dlnaServer.HTTPConn.Addr().String()
}

func TestInit(t *testing.T) {
	configfile.Install()

	f, err := fs.NewFs(context.Background(), "testdata/files")
	l, _ := f.List(context.Background(), "")
	fmt.Println(l)
	require.NoError(t, err)

	startServer(t, f)
}

// Make sure that it serves rootDesc.xml (SCPD in uPnP parlance).
func TestRootSCPD(t *testing.T) {
	req, err := http.NewRequest("GET", baseURL+rootDescPath, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
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
	req, err := http.NewRequest("GET", baseURL+resPath+"video.mp4", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer fs.CheckClose(resp.Body, &err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	actualContents, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	// Now compare the contents with the golden file.
	node, err := dlnaServer.vfs.Stat("/video.mp4")
	assert.NoError(t, err)
	goldenFile := node.(*vfs.File)
	goldenReader, err := goldenFile.Open(os.O_RDONLY)
	assert.NoError(t, err)
	defer fs.CheckClose(goldenReader, &err)
	goldenContents, err := io.ReadAll(goldenReader)
	assert.NoError(t, err)

	require.Equal(t, goldenContents, actualContents)
}

// Check that ContentDirectory#Browse returns appropriate metadata on the root container.
func TestContentDirectoryBrowseMetadata(t *testing.T) {
	// Sample from: https://github.com/rclone/rclone/issues/3253#issuecomment-524317469
	req, err := http.NewRequest("POST", baseURL+serviceControlURL, strings.NewReader(`
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
            <ObjectID>0</ObjectID>
            <BrowseFlag>BrowseMetadata</BrowseFlag>
            <Filter>*</Filter>
            <StartingIndex>0</StartingIndex>
            <RequestedCount>0</RequestedCount>
            <SortCriteria></SortCriteria>
        </u:Browse>
    </s:Body>
</s:Envelope>`))
	require.NoError(t, err)
	req.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// should contain an appropriate URN
	require.Contains(t, string(body), "urn:schemas-upnp-org:service:ContentDirectory:1")
	// expect a <container> element
	require.Contains(t, string(body), html.EscapeString("<container "))
	require.NotContains(t, string(body), html.EscapeString("<item "))
	// if there is a childCount, it better not be zero
	require.NotContains(t, string(body), html.EscapeString(" childCount=\"0\""))
	// should have a dc:date element
	require.Contains(t, string(body), html.EscapeString("<dc:date>"))
}

// Check that the X_MS_MediaReceiverRegistrar is faked out properly.
func TestMediaReceiverRegistrarService(t *testing.T) {
	env := soap.Envelope{
		Body: soap.Body{
			Action: []byte("RegisterDevice"),
		},
	}
	req, err := http.NewRequest("POST", baseURL+serviceControlURL, bytes.NewReader(mustMarshalXML(env)))
	require.NoError(t, err)
	req.Header.Set("SOAPACTION", `"urn:microsoft.com:service:X_MS_MediaReceiverRegistrar:1#RegisterDevice"`)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "<RegistrationRespMsg>")
}

// Check that ContentDirectory#Browse returns the expected items.
func TestContentDirectoryBrowseDirectChildren(t *testing.T) {
	// First the root...
	req, err := http.NewRequest("POST", baseURL+serviceControlURL, strings.NewReader(`
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
            <ObjectID>0</ObjectID>
            <BrowseFlag>BrowseDirectChildren</BrowseFlag>
            <Filter>*</Filter>
            <StartingIndex>0</StartingIndex>
            <RequestedCount>0</RequestedCount>
            <SortCriteria></SortCriteria>
        </u:Browse>
    </s:Body>
</s:Envelope>`))
	require.NoError(t, err)
	req.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// expect video.mp4, video.srt, video.en.srt URLs to be in the DIDL
	require.Contains(t, string(body), "/r/video.mp4")
	require.Contains(t, string(body), "/r/video.srt")
	require.Contains(t, string(body), "/r/video.en.srt")

	// Then a subdirectory
	req, err = http.NewRequest("POST", baseURL+serviceControlURL, strings.NewReader(`
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
            <ObjectID>%2Fsubdir</ObjectID>
            <BrowseFlag>BrowseDirectChildren</BrowseFlag>
            <Filter>*</Filter>
            <StartingIndex>0</StartingIndex>
            <RequestedCount>0</RequestedCount>
            <SortCriteria></SortCriteria>
        </u:Browse>
    </s:Body>
</s:Envelope>`))
	require.NoError(t, err)
	req.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	// expect video.mp4, video.srt, URLs to be in the DIDL
	require.Contains(t, string(body), "/r/subdir/video.mp4")
	require.Contains(t, string(body), "/r/subdir/video.srt")
}
