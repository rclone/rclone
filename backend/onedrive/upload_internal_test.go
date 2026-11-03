package onedrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// fragmentServer is a fake OneDrive upload session which records the
// fragments it receives. fail is called for each PUT before the body is
// accepted and may write an error response, returning true to reject it.
type fragmentServer struct {
	t         *testing.T
	size      int64
	mu        sync.Mutex
	received  []byte // bytes accepted so far
	fragments int    // number of accepted fragments
	fail      func(w http.ResponseWriter, attempt int) bool
	attempts  int
}

func (s *fragmentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == "POST" && r.URL.Path == "/root/createUploadSession":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"uploadUrl":"http://%s/upload","nextExpectedRanges":["0-"]}`, r.Host)
	case r.Method == "GET" && r.URL.Path == "/upload":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"nextExpectedRanges":["%d-%d"]}`, len(s.received), s.size-1)
	case r.Method == "PUT" && r.URL.Path == "/upload":
		body, err := io.ReadAll(r.Body)
		require.NoError(s.t, err)
		s.attempts++
		if s.fail != nil && s.fail(w, s.attempts) {
			return
		}
		var start, end, total int64
		_, err = fmt.Sscanf(r.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total)
		require.NoError(s.t, err)
		assert.Equal(s.t, int64(len(s.received)), start, "fragment sent out of order")
		assert.Equal(s.t, int64(len(body)), end-start+1, "Content-Range disagrees with body length")
		assert.Equal(s.t, s.size, total)
		s.received = append(s.received, body...)
		s.fragments++
		w.Header().Set("Content-Type", "application/json")
		if int64(len(s.received)) == s.size {
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"id":"fake-id","name":"remote","size":%d,"file":{}}`, s.size)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprintf(w, `{"nextExpectedRanges":["%d-%d"]}`, len(s.received), s.size-1)
	case r.Method == "DELETE" && r.URL.Path == "/upload":
		w.WriteHeader(http.StatusNoContent)
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// newUploadTest returns an Object wired to a fragmentServer which
// uploads content in chunks of chunkSize.
func newUploadTest(t *testing.T, content []byte, chunkSize int) (*Object, *fragmentServer, *httptest.Server) {
	s := &fragmentServer{t: t, size: int64(len(content))}
	server := httptest.NewServer(s)
	t.Cleanup(server.Close)
	client := rest.NewClient(server.Client()).SetRoot(server.URL)
	f := &Fs{
		ci:     fs.GetConfig(context.Background()),
		srv:    client,
		unAuth: client,
		pacer:  fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.opt.ChunkSize = fs.SizeSuffix(chunkSize)
	f.opt.UploadCutoff = fs.SizeSuffix(chunkSize)
	return &Object{fs: f}, s, server
}

// TestUploadMultipartSkip checks that a 416 response makes the upload
// read the server's position and re-send only the unreceived tail of
// the fragment.
func TestUploadMultipartSkip(t *testing.T) {
	content := bytes.Repeat([]byte("onedrive"), 1024)
	chunkSize := len(content) / 2
	skip := chunkSize / 4
	o, s, _ := newUploadTest(t, content, chunkSize)
	s.fail = func(w http.ResponseWriter, attempt int) bool {
		if attempt == 1 {
			// Pretend the first part of the fragment was received
			// before the connection broke
			s.received = append(s.received, content[:skip]...)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "invalidRange", "message": "bad range"}})
			return true
		}
		return false
	}
	src := object.NewStaticObjectInfo("remote", time.Now(), int64(len(content)), true, nil, nil)
	info, err := o.uploadMultipart(context.Background(), bytes.NewReader(content), src)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, content, s.received)
	assert.Equal(t, 2, s.fragments)
	assert.Equal(t, 3, s.attempts, "expected one 416, a partial re-send and the final fragment")
}

// TestURLAuthFor checks that tokens are only sent to HTTPS URLs on
// the API host or a SharePoint host which aren't pre-authenticated.
func TestURLAuthFor(t *testing.T) {
	const tenant = "https://contoso-my.sharepoint.com/_api"
	for _, tc := range []struct {
		name      string
		region    string
		tenantURL string
		url       string
		want      urlAuth
	}{
		{"graph", "global", "", "https://graph.microsoft.com/v1.0/drives/x/items/y/uploadSession", urlAuthAPI},
		{"graph case", "global", "", "https://Graph.Microsoft.com/v1.0/monitor/abc", urlAuthAPI},
		{"graph tempauth", "global", "", "https://graph.microsoft.com/v1.0/upload?tempauth=abc", urlAuthNone},
		{"graph http", "global", "", "http://graph.microsoft.com/v1.0/upload", urlAuthNone},
		{"graph other region", "global", "", "https://graph.microsoft.us/v1.0/upload", urlAuthNone},
		{"graph us", "us", "", "https://graph.microsoft.us/v1.0/upload", urlAuthAPI},
		{"graph lookalike", "global", "", "https://graph.microsoft.com.example.com/upload", urlAuthNone},
		{"sharepoint tempauth", "global", "", "https://contoso-my.sharepoint.com/_api/v2.0/upload?guid=x&tempauth=abc", urlAuthNone},
		{"sharepoint", "global", "", "https://contoso-my.sharepoint.com/_api/v2.0/upload?guid=x", urlAuthSharePoint},
		{"sharepoint case", "global", "", "https://Contoso.SharePoint.com/_api/v2.0/monitor/abc", urlAuthSharePoint},
		{"sharepoint us", "us", "", "https://contoso.sharepoint.us/_api/v2.0/upload", urlAuthSharePoint},
		{"sharepoint cn", "cn", "", "https://contoso.sharepoint.cn/_api/v2.0/upload", urlAuthSharePoint},
		{"sharepoint http", "global", "", "http://contoso-my.sharepoint.com/_api/v2.0/upload", urlAuthNone},
		{"sharepoint lookalike", "global", "", "https://contoso.sharepoint.com.example.com/upload", urlAuthNone},
		{"sharepoint suffix only", "global", "", "https://evilsharepoint.com/upload", urlAuthNone},
		{"personal", "global", "", "https://api.onedrive.com/rup/abc", urlAuthNone},
		{"personal upload", "global", "", "https://my.microsoftpersonalcontent.com/personal/abc/_api/v2.0/drive/items/x/uploadSession", urlAuthNone},
		{"unparseable", "global", "", "https://graph.microsoft.com/%zz", urlAuthNone},
		{"tenant", "global", tenant, "https://contoso-my.sharepoint.com/_api/v2.0/monitor/abc", urlAuthAPI},
		{"tenant tempauth", "global", tenant, "https://contoso-my.sharepoint.com/_api/v2.0/upload?tempauth=abc", urlAuthNone},
		{"tenant graph", "global", tenant, "https://graph.microsoft.com/v1.0/upload", urlAuthNone},
		{"tenant other sharepoint", "global", tenant, "https://contoso.sharepoint.com/_api/v2.0/upload", urlAuthSharePoint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &Fs{}
			f.opt.Region = tc.region
			f.opt.TenantURL = tc.tenantURL
			assert.Equal(t, tc.want, f.urlAuthFor(tc.url))
		})
	}
}

// TestCheckSharePointRedirect checks the SharePoint token is only
// sent on redirects to SharePoint over HTTPS.
func TestCheckSharePointRedirect(t *testing.T) {
	for _, tc := range []struct {
		url string
		ok  bool
	}{
		{"https://contoso-my.sharepoint.com/download.aspx", true},
		{"https://205807-ipv4v6.gr.global.aa-rt.sharepoint.com/download.aspx?tempauth=x", true},
		{"https://contoso.sharepoint.us/download.aspx", true},
		{"http://contoso-my.sharepoint.com/download.aspx", false},
		{"https://contoso.sharepoint.com.example.com/download.aspx", false},
		{"https://graph.microsoft.com/v1.0/me", false},
		{"https://example.com/", false},
	} {
		req, err := http.NewRequest("GET", tc.url, nil)
		require.NoError(t, err)
		err = checkSharePointRedirect(req, nil)
		if tc.ok {
			assert.NoError(t, err, tc.url)
		} else {
			assert.Error(t, err, tc.url)
		}
	}
	req, err := http.NewRequest("GET", "https://contoso-my.sharepoint.com/", nil)
	require.NoError(t, err)
	assert.Error(t, checkSharePointRedirect(req, make([]*http.Request, 10)), "too many redirects")
}

// TestCheckPreauthRetired checks which 401 errors get the explanation.
func TestCheckPreauthRetired(t *testing.T) {
	const sharePointURL = "https://contoso-my.sharepoint.com/_api/v2.0/upload?guid=x"
	errBase := errors.New("HTTP error 401")
	for _, tc := range []struct {
		name      string
		driveType string
		url       string
		status    int
		want      bool
	}{
		{"sharepoint 401", driveTypeBusiness, sharePointURL, http.StatusUnauthorized, true},
		{"library 401", driveTypeSharepoint, sharePointURL, http.StatusUnauthorized, true},
		{"personal 401", driveTypePersonal, sharePointURL, http.StatusUnauthorized, false},
		{"sharepoint 403", driveTypeBusiness, sharePointURL, http.StatusForbidden, false},
		{"tempauth 401", driveTypeBusiness, sharePointURL + "&tempauth=abc", http.StatusUnauthorized, false},
		{"graph 401", driveTypeBusiness, "https://graph.microsoft.com/v1.0/upload", http.StatusUnauthorized, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &Fs{driveType: tc.driveType}
			f.opt.Region = "global"
			err := f.checkPreauthRetired(tc.url, &http.Response{StatusCode: tc.status}, errBase)
			assert.ErrorIs(t, err, errBase)
			assert.Equal(t, tc.want, errors.Is(err, errPreauthRetired), "err = %v", err)
		})
	}
	f := &Fs{driveType: driveTypeBusiness}
	assert.NoError(t, f.checkPreauthRetired(sharePointURL, &http.Response{StatusCode: http.StatusUnauthorized}, nil))
	assert.ErrorIs(t, f.checkPreauthRetired(sharePointURL, nil, errBase), errBase)
}

// TestSharePointClient checks that the client for a SharePoint host
// is authenticated with a token for that host got from the existing
// refresh token, which is fetched once and not stored.
func TestSharePointClient(t *testing.T) {
	var scopes []string
	var auths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			assert.Equal(t, "refresh_token", r.PostForm.Get("grant_type"))
			assert.Equal(t, "refresh", r.PostForm.Get("refresh_token"))
			scopes = append(scopes, r.PostForm.Get("scope"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"access_token":"sharepoint_token","token_type":"Bearer","expires_in":3600}`)
		case "/upload":
			auths = append(auths, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"nextExpectedRanges":["0-"]}`)
		case "/redirect":
			http.Redirect(w, r, "/upload", http.StatusFound)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
	}))
	t.Cleanup(server.Close)

	storedToken := `{"access_token":"graph_token","token_type":"Bearer","refresh_token":"refresh","expiry":"2099-01-01T00:00:00Z"}`
	m := configmap.Simple{"token": storedToken}
	ctx := context.Background()
	_, ts, err := oauthutil.NewClientWithBaseClient(ctx, "remote", m, &oauthutil.Config{
		ClientID:     "client_id",
		ClientSecret: "client_secret",
		TokenURL:     server.URL + "/token",
	}, server.Client())
	require.NoError(t, err)
	f := &Fs{tokenSource: ts, sharePoint: &sharePointClients{hosts: make(map[string]sharePointAuth)}}

	for range 2 {
		client, err := f.sharePointClient(ctx, "Contoso-my.sharepoint.com")
		require.NoError(t, err)
		_, err = client.Call(ctx, &rest.Opts{Method: "GET", RootURL: server.URL + "/upload"})
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"https://contoso-my.sharepoint.com/.default"}, scopes, "token should be fetched once for the host")
	assert.Equal(t, []string{"Bearer sharepoint_token", "Bearer sharepoint_token"}, auths)
	assert.Equal(t, storedToken, m["token"], "the stored token must not change")

	// The token isn't sent on a redirect to a host which isn't SharePoint
	client, err := f.sharePointClient(ctx, "contoso-my.sharepoint.com")
	require.NoError(t, err)
	_, err = client.Call(ctx, &rest.Opts{Method: "GET", RootURL: server.URL + "/redirect"})
	assert.ErrorContains(t, err, "refusing to send a SharePoint token")
	assert.Len(t, auths, 2)

	// Without credentials there is no client and the error explains why
	_, err = (&Fs{}).sharePointClient(ctx, "contoso-my.sharepoint.com")
	assert.ErrorIs(t, err, errPreauthRetired)
}

// TestUploadSessionFallback checks that an upload session which needs
// SharePoint authentication which can't be got is replaced by a single
// part upload before any data is read.
func TestUploadSessionFallback(t *testing.T) {
	content := bytes.Repeat([]byte("onedrive"), 1024)
	var requests []string
	var uploaded []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/root/createUploadSession":
			_, _ = fmt.Fprint(w, `{"uploadUrl":"https://contoso-my.sharepoint.com/_api/v2.0/upload?guid=x","nextExpectedRanges":["0-"]}`)
		case r.Method == "PUT" && r.URL.Path == "/root/content":
			var err error
			uploaded, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			_, _ = fmt.Fprintf(w, `{"id":"fake-id","name":"remote","size":%d,"file":{}}`, len(uploaded))
		case r.Method == "PATCH":
			_, _ = fmt.Fprintf(w, `{"id":"fake-id","name":"remote","size":%d,"file":{}}`, len(uploaded))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := rest.NewClient(server.Client()).SetRoot(server.URL)
	f := &Fs{
		ci:        fs.GetConfig(context.Background()),
		srv:       client,
		unAuth:    client,
		driveType: driveTypeBusiness,
		pacer:     fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.opt.Region = "global"
	f.opt.ChunkSize = fs.SizeSuffix(len(content) / 2)
	o := &Object{fs: f}

	src := object.NewStaticObjectInfo("remote", time.Now(), int64(len(content)), true, nil, nil)
	_, err := o.uploadMultipart(context.Background(), bytes.NewReader(content), src)
	require.NoError(t, err)
	assert.Equal(t, content, uploaded)
	assert.Equal(t, []string{"POST /root/createUploadSession", "PUT /root/content"}, requests[:2])
}

// redirectTransport sends every request to the test server, so URLs
// on other hosts can be tested.
type redirectTransport struct {
	server *httptest.Server
}

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return t.server.Client().Transport.RoundTrip(req)
}

// newDownloadTest returns an Object which downloads content from a
// server which handles the /content call with contentHandler and
// serves download.aspx with Range support, recording the
// Authorization header of each download.
func newDownloadTest(t *testing.T, content []byte, contentHandler http.HandlerFunc) (o *Object, auths *[]string) {
	auths = new([]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/items/fake/content":
			contentHandler(w, r)
		case "/download.aspx":
			*auths = append(*auths, r.Header.Get("Authorization"))
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(content))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	transport := redirectTransport{server: server}
	sharePointToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "sharepoint_token"})
	sharePointHTTP := &http.Client{Transport: &oauth2.Transport{Source: sharePointToken, Base: transport}}
	f := &Fs{
		ci:          fs.GetConfig(context.Background()),
		srv:         rest.NewClient(server.Client()).SetRoot(server.URL),
		unAuth:      rest.NewClient(&http.Client{Transport: transport}),
		driveType:   driveTypeBusiness,
		tokenSource: &oauthutil.TokenSource{},
		sharePoint: &sharePointClients{hosts: map[string]sharePointAuth{
			"contoso-my.sharepoint.com": {client: rest.NewClient(sharePointHTTP), tokenSource: sharePointToken},
		}},
		pacer: fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.opt.Region = "global"
	return &Object{fs: f, id: "fake", size: int64(len(content))}, auths
}

// TestOpenRedirectAuth checks that downloads are redirected to a
// pre-authenticated URL without authentication and to a SharePoint
// URL with a SharePoint token.
func TestOpenRedirectAuth(t *testing.T) {
	content := []byte("0123456789abcdefghij")
	for _, test := range []struct {
		name     string
		location string
		wantAuth string
	}{
		{"tempauth", "https://contoso-my.sharepoint.com/download.aspx?UniqueId=x&tempauth=v1.secret", ""},
		{"sharepoint", "https://contoso-my.sharepoint.com/download.aspx?UniqueId=x", "Bearer sharepoint_token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			o, auths := newDownloadTest(t, content, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, test.location, http.StatusFound)
			})
			in, err := o.Open(context.Background(), &fs.RangeOption{Start: 5, End: 9})
			require.NoError(t, err)
			got, err := io.ReadAll(in)
			require.NoError(t, err)
			require.NoError(t, in.Close())
			assert.Equal(t, "56789", string(got))
			assert.Equal(t, []string{test.wantAuth}, *auths)
		})
	}

	// Without credentials the error explains why
	o, auths := newDownloadTest(t, content, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://contoso-my.sharepoint.com/download.aspx?UniqueId=x", http.StatusFound)
	})
	o.fs.tokenSource = nil
	_, err := o.Open(context.Background())
	assert.ErrorIs(t, err, errPreauthRetired)
	assert.Empty(t, *auths)
}
