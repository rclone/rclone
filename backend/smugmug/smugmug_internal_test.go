package smugmug

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/smugmug/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
)

func TestAlbumPathFromURLPath(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"/Galleries/Blue-Mesa", "/Galleries/Blue-Mesa"},
		{"/Galleries/Blue-Mesa/i-AbCdEf1/A", "/Galleries/Blue-Mesa"},
		{"Galleries/Blue-Mesa/i-AbCdEf1", "/Galleries/Blue-Mesa"},
		{"/", "/"},
	} {
		got := albumPathFromURLPath(test.in)
		if got != test.want {
			t.Fatalf("albumPathFromURLPath(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestNormalizeAlbumURI(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"AbCdEf", "/api/v2/album/AbCdEf"},
		{"/album/AbCdEf", "/api/v2/album/AbCdEf"},
		{"/api/v2/album/AbCdEf", "/api/v2/album/AbCdEf"},
	} {
		got, err := normalizeAlbumURI(test.in)
		if err != nil {
			t.Fatalf("normalizeAlbumURI(%q) returned error: %v", test.in, err)
		}
		if got != test.want {
			t.Fatalf("normalizeAlbumURI(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestNormalizeNodeURI(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"NdAbCd", "/api/v2/node/NdAbCd"},
		{"/node/NdAbCd", "/api/v2/node/NdAbCd"},
		{"/api/v2/node/NdAbCd", "/api/v2/node/NdAbCd"},
		{"https://api.smugmug.com/api/v2/node/NdAbCd", "/api/v2/node/NdAbCd"},
	} {
		got, err := normalizeNodeURI(test.in)
		if err != nil {
			t.Fatalf("normalizeNodeURI(%q) returned error: %v", test.in, err)
		}
		if got != test.want {
			t.Fatalf("normalizeNodeURI(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestRevealObscured(t *testing.T) {
	want := strings.Repeat("s", 64)
	got, err := revealObscured("api_secret", obscure.MustObscure(want))
	if err != nil {
		t.Fatalf("revealObscured returned error: %v", err)
	}
	if got != want {
		t.Fatalf("revealObscured = %q, want %q", got, want)
	}
}

func TestRevealObscuredAcceptsShortSecret(t *testing.T) {
	want := "short-secret"
	got, err := revealObscured("api_secret", obscure.MustObscure(want))
	if err != nil {
		t.Fatalf("revealObscured returned error: %v", err)
	}
	if got != want {
		t.Fatalf("revealObscured = %q, want %q", got, want)
	}
}

func TestRevealObscuredRejectsPlainAPISecret(t *testing.T) {
	plainBase64URLSecret := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789AB"
	_, err := revealObscured("api_secret", plainBase64URLSecret)
	if err == nil {
		t.Fatal("expected plain API secret to be rejected")
	}
}

func TestGetOptionsDefaultsToRootNode(t *testing.T) {
	opt, err := getOptions(configmap.Simple{})
	if err != nil {
		t.Fatalf("getOptions returned error: %v", err)
	}
	if opt.RootNode != "root" {
		t.Fatalf("RootNode = %q, want %q", opt.RootNode, "root")
	}
	if opt.AlbumURI != "" {
		t.Fatalf("AlbumURI = %q, want empty", opt.AlbumURI)
	}
}

func TestGetOptionsKeepsAlbumMode(t *testing.T) {
	opt, err := getOptions(configmap.Simple{
		"album_uri": "/api/v2/album/AbCdEf",
	})
	if err != nil {
		t.Fatalf("getOptions returned error: %v", err)
	}
	if opt.AlbumURI != "/api/v2/album/AbCdEf" {
		t.Fatalf("AlbumURI = %q, want %q", opt.AlbumURI, "/api/v2/album/AbCdEf")
	}
	if opt.RootNode != "" {
		t.Fatalf("RootNode = %q, want empty", opt.RootNode)
	}
}

func TestConfigExistingTokenAsksRefresh(t *testing.T) {
	m := configmap.Simple{
		configAccessToken:     "token",
		"access_token_secret": obscure.MustObscure("secret"),
	}
	out, err := Config(context.Background(), "smug", m, fs.ConfigIn{})
	if err != nil {
		t.Fatalf("Config returned error: %v", err)
	}
	if out == nil || out.Option == nil {
		t.Fatalf("Config returned %#v, want refresh question", out)
	}
	if out.State != "refresh" {
		t.Fatalf("State = %q, want %q", out.State, "refresh")
	}
	if out.Option.Name != "config_refresh_token" {
		t.Fatalf("Option.Name = %q, want %q", out.Option.Name, "config_refresh_token")
	}
}

func TestConfigExistingTokenCanBeKept(t *testing.T) {
	m := configmap.Simple{
		configAccessToken:     "token",
		"access_token_secret": obscure.MustObscure("secret"),
	}
	out, err := Config(context.Background(), "smug", m, fs.ConfigIn{
		State:  "refresh",
		Result: "false",
	})
	if err != nil {
		t.Fatalf("Config returned error: %v", err)
	}
	if out != nil {
		t.Fatalf("Config returned %#v, want nil", out)
	}
}

func TestNewFsEmptyDirectoryFeature(t *testing.T) {
	ctx := context.Background()
	baseConfig := configmap.Simple{
		configAccessToken:     "token",
		"access_token_secret": obscure.MustObscure("0123456789abcdef"),
	}

	libraryConfig := configmap.Simple{}
	for key, value := range baseConfig {
		libraryConfig[key] = value
	}
	libraryConfig["root_node"] = "NdRoot"
	f, err := NewFs(ctx, "smug", "", libraryConfig)
	if err != nil {
		t.Fatalf("NewFs library mode returned error: %v", err)
	}
	if !f.Features().CanHaveEmptyDirectories {
		t.Fatal("library mode should advertise CanHaveEmptyDirectories")
	}

	albumConfig := configmap.Simple{}
	for key, value := range baseConfig {
		albumConfig[key] = value
	}
	albumConfig["album_uri"] = "/api/v2/album/AbCdEf"
	f, err = NewFs(ctx, "smug", "", albumConfig)
	if err != nil {
		t.Fatalf("NewFs album mode returned error: %v", err)
	}
	if f.Features().CanHaveEmptyDirectories {
		t.Fatal("album mode should not advertise CanHaveEmptyDirectories")
	}
}

func TestMkdirExistingLibraryPath(t *testing.T) {
	for _, test := range []struct {
		name        string
		dir         string
		fullDir     string
		loc         *libraryLocation
		errContains string
	}{
		{
			name:    "folder",
			fullDir: "Projects",
			loc:     &libraryLocation{node: api.Node{Type: "Folder"}},
		},
		{
			name:    "album",
			fullDir: "Projects/BlueMesa",
			loc:     &libraryLocation{node: api.Node{Type: "Album"}},
		},
		{
			name:    "virtual root",
			fullDir: "Projects/BlueMesa/prints",
			loc:     &libraryLocation{node: api.Node{Type: "Album"}, albumPrefix: "prints"},
		},
		{
			name:        "virtual child",
			dir:         "Projects/BlueMesa/prints",
			fullDir:     "Projects/BlueMesa/prints",
			loc:         &libraryLocation{node: api.Node{Type: "Album"}, albumPrefix: "prints"},
			errContains: "virtual",
		},
		{
			name:        "unsupported node",
			fullDir:     "Projects/Page",
			loc:         &libraryLocation{node: api.Node{Type: "Page"}},
			errContains: "not a folder",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := mkdirExistingLibraryPath(test.dir, test.fullDir, test.loc)
			if test.errContains == "" {
				if err != nil {
					t.Fatalf("mkdirExistingLibraryPath returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), test.errContains) {
				t.Fatalf("error %q does not contain %q", err, test.errContains)
			}
		})
	}
}

func TestCommandNodeInfoInParent(t *testing.T) {
	f := &Fs{}
	item := api.Node{
		Name:   "RiverLight",
		Type:   "Album",
		URI:    "/api/v2/node/NdAlbum",
		WebURI: "https://example.invalid/RiverLight",
		Uris: map[string]api.Link{
			"Album": {URI: "/api/v2/album/AbCdEf"},
		},
	}

	got := f.commandNodeInfoInParent(item, "Projects")
	if got.Path != "Projects/RiverLight" {
		t.Fatalf("Path = %q, want %q", got.Path, "Projects/RiverLight")
	}

	got = f.commandNodeInfoInParent(item, "")
	if got.Path != "RiverLight" {
		t.Fatalf("Path = %q, want %q", got.Path, "RiverLight")
	}
}

func TestResolveLibraryPathCachesNodes(t *testing.T) {
	ctx := context.Background()
	var rootGets, rootChildren, projectChildren int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/node/NdRoot", func(w http.ResponseWriter, r *http.Request) {
		rootGets++
		_, _ = w.Write([]byte(`{"Response":{"Node":{"Name":"Root","Type":"Folder","Uri":"/api/v2/node/NdRoot"}}}`))
	})
	mux.HandleFunc("/api/v2/node/NdRoot!children", func(w http.ResponseWriter, r *http.Request) {
		rootChildren++
		_, _ = w.Write([]byte(`{"Response":{"Node":[{"Name":"Projects","Type":"Folder","Uri":"/api/v2/node/NdProjects"}]}}`))
	})
	mux.HandleFunc("/api/v2/node/NdProjects!children", func(w http.ResponseWriter, r *http.Request) {
		projectChildren++
		_, _ = w.Write([]byte(`{"Response":{"Node":[{"Name":"BlueMesa","Type":"Album","Uri":"/api/v2/node/NdAlbum","Uris":{"Album":{"Uri":"/api/v2/album/AbCdEf"}}}]}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	f := &Fs{
		rootNodeURI: "/api/v2/node/NdRoot",
		client:      server.Client(),
		srv:         newSmugMugRESTClient(server.Client()).SetRoot(server.URL),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault()),
	}
	f.dirCache = dircache.New("", f.rootNodeURI, f)

	for range 2 {
		loc, err := f.resolveLibraryPath(ctx, "Projects/BlueMesa/photo.jpg")
		if err != nil {
			t.Fatalf("resolveLibraryPath returned error: %v", err)
		}
		if loc.albumURI != "/api/v2/album/AbCdEf" || loc.albumPrefix != "photo.jpg" {
			t.Fatalf("resolveLibraryPath returned albumURI=%q albumPrefix=%q", loc.albumURI, loc.albumPrefix)
		}
	}
	if rootGets != 1 || rootChildren != 1 || projectChildren != 1 {
		t.Fatalf("API calls = root:%d root children:%d project children:%d, want 1 each", rootGets, rootChildren, projectChildren)
	}
	if got, ok := f.dirCache.Get("Projects"); !ok || got != "/api/v2/node/NdProjects" {
		t.Fatalf("dir cache Projects = %q, %v; want /api/v2/node/NdProjects, true", got, ok)
	}
}

func TestAPILinkUnmarshal(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{`"/api/v2/node/NdAbCd"`, "/api/v2/node/NdAbCd"},
		{`{"Uri":"/api/v2/node/NdAbCd"}`, "/api/v2/node/NdAbCd"},
	} {
		var got api.Link
		if err := json.Unmarshal([]byte(test.in), &got); err != nil {
			t.Fatalf("json.Unmarshal(%s) returned error: %v", test.in, err)
		}
		if got.URI != test.want {
			t.Fatalf("json.Unmarshal(%s) = %q, want %q", test.in, got.URI, test.want)
		}
	}
}

func TestAlbumImageAPILinkUnmarshal(t *testing.T) {
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "string",
			in:   `{"Uris":{"Image":"/api/v2/image/AbCdEf"}}`,
			want: "/api/v2/image/AbCdEf",
		},
		{
			name: "object",
			in:   `{"Uris":{"Image":{"Uri":"/api/v2/image/AbCdEf"}}}`,
			want: "/api/v2/image/AbCdEf",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got api.AlbumImage
			if err := json.Unmarshal([]byte(test.in), &got); err != nil {
				t.Fatalf("json.Unmarshal(%s) returned error: %v", test.in, err)
			}
			if got.Uris["Image"].URI != test.want {
				t.Fatalf("json.Unmarshal(%s) = %q, want %q", test.in, got.Uris["Image"].URI, test.want)
			}
		})
	}
}

func TestAlbumImageMetadataUnmarshal(t *testing.T) {
	for _, test := range []struct {
		name          string
		in            string
		wantKeywords  string
		wantLatitude  float64
		wantHasLat    bool
		wantHidden    bool
		wantHasHidden bool
		wantTitle     string
		wantCaption   string
		wantLongitude float64
		wantHasLong   bool
		wantAltitude  float64
		wantHasAlt    bool
	}{
		{
			name:          "metadata values",
			in:            `{"Title":"Cover","Caption":"Trail","Keywords":["travel","landscape"],"Hidden":false,"Latitude":"35.681236","Longitude":139.767125,"Altitude":"12.5"}`,
			wantKeywords:  "travel,landscape",
			wantLatitude:  35.681236,
			wantHasLat:    true,
			wantHidden:    false,
			wantHasHidden: true,
			wantTitle:     "Cover",
			wantCaption:   "Trail",
			wantLongitude: 139.767125,
			wantHasLong:   true,
			wantAltitude:  12.5,
			wantHasAlt:    true,
		},
		{
			name:         "string keywords",
			in:           `{"Keywords":"travel,landscape"}`,
			wantKeywords: "travel,landscape",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got api.AlbumImage
			if err := json.Unmarshal([]byte(test.in), &got); err != nil {
				t.Fatalf("json.Unmarshal(%s) returned error: %v", test.in, err)
			}
			if string(got.Keywords) != test.wantKeywords {
				t.Fatalf("Keywords = %q, want %q", got.Keywords, test.wantKeywords)
			}
			if got.Title != test.wantTitle {
				t.Fatalf("Title = %q, want %q", got.Title, test.wantTitle)
			}
			if got.Caption != test.wantCaption {
				t.Fatalf("Caption = %q, want %q", got.Caption, test.wantCaption)
			}
			if got.Hidden != nil != test.wantHasHidden {
				t.Fatalf("Hidden present = %v, want %v", got.Hidden != nil, test.wantHasHidden)
			}
			if got.Hidden != nil && *got.Hidden != test.wantHidden {
				t.Fatalf("Hidden = %v, want %v", *got.Hidden, test.wantHidden)
			}
			latitude, hasLatitude := got.Latitude.Value()
			if hasLatitude != test.wantHasLat {
				t.Fatalf("Latitude present = %v, want %v", hasLatitude, test.wantHasLat)
			}
			if hasLatitude && latitude != test.wantLatitude {
				t.Fatalf("Latitude = %v, want %v", latitude, test.wantLatitude)
			}
			longitude, hasLongitude := got.Longitude.Value()
			if hasLongitude != test.wantHasLong {
				t.Fatalf("Longitude present = %v, want %v", hasLongitude, test.wantHasLong)
			}
			if hasLongitude && longitude != test.wantLongitude {
				t.Fatalf("Longitude = %v, want %v", longitude, test.wantLongitude)
			}
			altitude, hasAltitude := got.Altitude.Value()
			if hasAltitude != test.wantHasAlt {
				t.Fatalf("Altitude present = %v, want %v", hasAltitude, test.wantHasAlt)
			}
			if hasAltitude && altitude != test.wantAltitude {
				t.Fatalf("Altitude = %v, want %v", altitude, test.wantAltitude)
			}
		})
	}
}

func TestNewObjectUsesArchivedRenditionMetadata(t *testing.T) {
	f := &Fs{}
	const md5sum = "0123456789abcdef0123456789abcdef"
	o := f.newObjectFromImageInAlbum("photo.jpg", api.AlbumImage{
		URI:          "/api/v2/album/AbCdEf/image/ImgOne",
		FileName:     "photo.jpg",
		ArchivedURI:  "https://example.invalid/archived.jpg",
		ArchivedSize: 7,
		ArchivedMD5:  strings.ToUpper(md5sum),
		OriginalSize: 99,
		Size:         88,
	}, "/api/v2/album/AbCdEf", "photo.jpg")

	if o.Size() != 7 {
		t.Fatalf("Size = %d, want 7", o.Size())
	}
	gotHash, err := o.Hash(context.Background(), hash.MD5)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}
	if gotHash != md5sum {
		t.Fatalf("Hash = %q, want %q", gotHash, md5sum)
	}
	if o.downloadURL != "https://example.invalid/archived.jpg" {
		t.Fatalf("downloadURL = %q, want archived URL", o.downloadURL)
	}
}

func TestNewObjectUnknownSizeIsZero(t *testing.T) {
	o := (&Fs{}).newObjectFromImageInAlbum("photo.jpg", api.AlbumImage{
		URI:      "/api/v2/album/AbCdEf/image/ImgOne",
		FileName: "photo.jpg",
	}, "/api/v2/album/AbCdEf", "photo.jpg")
	if o.Size() != 0 {
		t.Fatalf("Size = %d, want 0", o.Size())
	}
}

func TestSmugMugMetadataPatch(t *testing.T) {
	patch, err := smugMugMetadataPatch(fs.Metadata{
		"title":     "Cover",
		"caption":   "Trail",
		"keywords":  "travel,landscape",
		"hidden":    "true",
		"latitude":  "35.681236",
		"longitude": "139.767125",
		"altitude":  "12.5",
		"unknown":   "ignored",
	})
	if err != nil {
		t.Fatalf("smugMugMetadataPatch returned error: %v", err)
	}
	for key, want := range map[string]any{
		"Title":     "Cover",
		"Caption":   "Trail",
		"Keywords":  "travel,landscape",
		"Hidden":    true,
		"Latitude":  35.681236,
		"Longitude": 139.767125,
		"Altitude":  12.5,
	} {
		if patch[key] != want {
			t.Fatalf("patch[%q] = %#v, want %#v", key, patch[key], want)
		}
	}
	if _, ok := patch["unknown"]; ok {
		t.Fatal("unknown metadata key was not ignored")
	}
}

func TestApplySmugMugUploadMetadata(t *testing.T) {
	headers := map[string]string{}
	err := applySmugMugUploadMetadata(headers, fs.Metadata{
		"title":     "Cover",
		"caption":   "Trail",
		"keywords":  "travel,landscape",
		"hidden":    "false",
		"latitude":  "35.681236",
		"longitude": "139.767125",
		"altitude":  "12.5",
	})
	if err != nil {
		t.Fatalf("applySmugMugUploadMetadata returned error: %v", err)
	}
	for key, want := range map[string]string{
		"X-Smug-Title":     "Cover",
		"X-Smug-Caption":   "Trail",
		"X-Smug-Keywords":  "travel,landscape",
		"X-Smug-Hidden":    "false",
		"X-Smug-Latitude":  "35.681236",
		"X-Smug-Longitude": "139.767125",
		"X-Smug-Altitude":  "12.5",
	} {
		if headers[key] != want {
			t.Fatalf("headers[%q] = %q, want %q", key, headers[key], want)
		}
	}
}

func TestObjectOpenUsesUnsignedDownloadClient(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("download request sent Authorization header %q", got)
		}
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

	baseClient := server.Client()
	signedClient := *baseClient
	signedClient.Transport = &oauth1Transport{
		base: baseClient.Transport,
		cred: oauthCredentials{
			consumerKey:    "key",
			consumerSecret: "secret",
			token:          "token",
			tokenSecret:    "token-secret",
		},
	}
	f := &Fs{
		client:         &signedClient,
		downloadClient: baseClient,
		pacer:          fs.NewPacer(ctx, pacer.NewDefault()),
	}
	o := &Object{
		fs:          f,
		downloadURL: server.URL,
	}

	in, err := o.Open(ctx)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer fs.CheckClose(in, &err)
	got, err := io.ReadAll(in)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != "image" {
		t.Fatalf("download body = %q, want %q", got, "image")
	}
}

func TestFindDownloadDetails(t *testing.T) {
	details := findDownloadDetails(map[string]any{
		"Response": map[string]any{
			"Image": map[string]any{
				"ArchivedUri":  "https://example.invalid/archive.jpg",
				"ArchivedSize": float64(12),
				"ArchivedMD5":  "0123456789abcdef0123456789abcdef",
			},
		},
	})
	if details.URL != "https://example.invalid/archive.jpg" {
		t.Fatalf("URL = %q, want archived URL", details.URL)
	}
	if details.Size != 12 {
		t.Fatalf("Size = %d, want 12", details.Size)
	}
	if details.MD5 != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("MD5 = %q, want archived MD5", details.MD5)
	}
}

func TestUploadWithNonSeekableBodyReturnsHTTPError(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"Code":500,"Message":"server said no"}`))
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	client := server.Client()
	client.Transport = rewriteTransport{
		base:   client.Transport,
		target: target,
	}
	f := &Fs{
		client: client,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault()),
	}

	err = f.upload(ctx, bytes.NewBufferString("body"), 4, nil, "", &api.UploadResponse{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "server said no") {
		t.Fatalf("upload error = %q, want SmugMug HTTP error body", err)
	}
}

func TestUploadRetriesSeekableAccountingBody(t *testing.T) {
	ctx := context.Background()
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll returned error: %v", err)
		}
		if string(body) != "body" {
			t.Errorf("request body = %q, want %q", body, "body")
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"Code":500,"Message":"retry me"}`))
			return
		}
		_, _ = w.Write([]byte(`{"stat":"ok","Image":{"ImageUri":"/api/v2/image/ImgOne","AlbumImageUri":"/api/v2/album/AbCdEf/image/ImgOne"}}`))
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	client := server.Client()
	client.Transport = rewriteTransport{
		base:   client.Transport,
		target: target,
	}
	f := &Fs{
		client: client,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault()),
	}

	var upload api.UploadResponse
	err = f.upload(ctx, &testAccounter{in: bytes.NewReader([]byte("body"))}, 4, nil, "", &upload)
	if err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if upload.Image.ImageURI != "/api/v2/image/ImgOne" {
		t.Fatalf("ImageURI = %q, want uploaded image URI", upload.Image.ImageURI)
	}
}

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.URL.Path = t.target.Path
	clone.URL.RawPath = t.target.RawPath
	clone.URL.RawQuery = t.target.RawQuery
	clone.Host = t.target.Host
	return base.RoundTrip(clone)
}

type testAccounter struct {
	in io.Reader
}

func (a *testAccounter) Read(p []byte) (int, error) {
	return a.in.Read(p)
}

func (a *testAccounter) OldStream() io.Reader {
	return a.in
}

func (a *testAccounter) SetStream(in io.Reader) {
	a.in = in
}

func (a *testAccounter) WrapStream(in io.Reader) io.Reader {
	return &testAccounter{in: in}
}

func TestListPreservesDuplicateFileNames(t *testing.T) {
	ctx := context.Background()
	server := duplicateImageServer(t)
	defer server.Close()

	f := &Fs{
		albumURI: server.URL + "/api/v2/album/AbCdEf",
		client:   server.Client(),
		pacer:    fs.NewPacer(ctx, pacer.NewDefault()),
	}
	entries, err := f.List(ctx, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	assertDuplicateImageEntries(t, entries)
}

func TestListAlbumEntriesPreservesDuplicateFileNames(t *testing.T) {
	ctx := context.Background()
	server := duplicateImageServer(t)
	defer server.Close()

	f := &Fs{
		client: server.Client(),
		pacer:  fs.NewPacer(ctx, pacer.NewDefault()),
	}
	entries, err := f.listAlbumEntries(ctx, "", server.URL+"/api/v2/album/AbCdEf", "")
	if err != nil {
		t.Fatalf("listAlbumEntries returned error: %v", err)
	}
	assertDuplicateImageEntries(t, entries)
}

func duplicateImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/album/AbCdEf!images" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response": {
				"AlbumImage": [
					{"Uri": "/api/v2/album/AbCdEf/image/ImgOne", "FileName": "photo.jpg", "OriginalSize": 1},
					{"Uri": "/api/v2/album/AbCdEf/image/ImgTwo", "FileName": "photo.jpg", "OriginalSize": 2}
				]
			}
		}`))
	}))
}

func assertDuplicateImageEntries(t *testing.T, entries fs.DirEntries) {
	t.Helper()
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	for i, entry := range entries {
		obj, ok := entry.(*Object)
		if !ok {
			t.Fatalf("entry %d has type %T, want *Object", i, entry)
		}
		if obj.Remote() != "photo.jpg" {
			t.Fatalf("entry %d remote = %q, want %q", i, obj.Remote(), "photo.jpg")
		}
		if obj.Size() != int64(i+1) {
			t.Fatalf("entry %d size = %d, want %d", i, obj.Size(), i+1)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name string
		in   string
		want time.Duration
		ok   bool
	}{
		{
			name: "seconds",
			in:   "3",
			want: 3 * time.Second,
			ok:   true,
		},
		{
			name: "http date",
			in:   now.Add(5 * time.Second).Format(http.TimeFormat),
			want: 5 * time.Second,
			ok:   true,
		},
		{
			name: "past date",
			in:   now.Add(-5 * time.Second).Format(http.TimeFormat),
			want: 0,
			ok:   true,
		},
		{
			name: "bad",
			in:   "later",
			ok:   false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseRetryAfter(test.in, now)
			if ok != test.ok {
				t.Fatalf("parseRetryAfter(%q) ok = %v, want %v", test.in, ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("parseRetryAfter(%q) = %v, want %v", test.in, got, test.want)
			}
		})
	}
}

func TestFirstAlbumURIFromPage(t *testing.T) {
	page := normalizeSmugMugPageText(`"Uris":{"Album":{"Uri":"\/api\/v2\/album\/AbCdEf"}}`)
	if got, want := firstAlbumURI(page), "/api/v2/album/AbCdEf"; got != want {
		t.Fatalf("firstAlbumURI() = %q, want %q", got, want)
	}
}

func TestCommandImageTransferArgs(t *testing.T) {
	for _, test := range []struct {
		name    string
		arg     []string
		opt     map[string]string
		wantSrc string
		wantDst string
		wantErr bool
	}{
		{
			name:    "positional",
			arg:     []string{"Projects/BlueMesa/photo.jpg", "Projects/RiverLight/photo.jpg"},
			wantSrc: "Projects/BlueMesa/photo.jpg",
			wantDst: "Projects/RiverLight/photo.jpg",
		},
		{
			name: "options",
			opt: map[string]string{
				"src": "Projects/BlueMesa/photo.jpg",
				"dst": "Projects/RiverLight/photo.jpg",
			},
			wantSrc: "Projects/BlueMesa/photo.jpg",
			wantDst: "Projects/RiverLight/photo.jpg",
		},
		{
			name: "mixed",
			arg:  []string{"Projects/RiverLight/photo.jpg"},
			opt: map[string]string{
				"src": "Projects/BlueMesa/photo.jpg",
			},
			wantSrc: "Projects/BlueMesa/photo.jpg",
			wantDst: "Projects/RiverLight/photo.jpg",
		},
		{
			name:    "missing",
			wantErr: true,
		},
		{
			name:    "extra",
			arg:     []string{"a.jpg", "b.jpg", "c.jpg"},
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotSrc, gotDst, err := commandImageTransferArgs(test.arg, test.opt)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("commandImageTransferArgs() returned error: %v", err)
			}
			if gotSrc != test.wantSrc || gotDst != test.wantDst {
				t.Fatalf("commandImageTransferArgs() = %q, %q; want %q, %q", gotSrc, gotDst, test.wantSrc, test.wantDst)
			}
		})
	}
}
