package smugmug

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
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

func TestAPILinkUnmarshal(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{`"/api/v2/node/NdAbCd"`, "/api/v2/node/NdAbCd"},
		{`{"Uri":"/api/v2/node/NdAbCd"}`, "/api/v2/node/NdAbCd"},
	} {
		var got apiLink
		if err := json.Unmarshal([]byte(test.in), &got); err != nil {
			t.Fatalf("json.Unmarshal(%s) returned error: %v", test.in, err)
		}
		if got.Uri != test.want {
			t.Fatalf("json.Unmarshal(%s) = %q, want %q", test.in, got.Uri, test.want)
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
			var got albumImage
			if err := json.Unmarshal([]byte(test.in), &got); err != nil {
				t.Fatalf("json.Unmarshal(%s) returned error: %v", test.in, err)
			}
			if got.Uris["Image"].Uri != test.want {
				t.Fatalf("json.Unmarshal(%s) = %q, want %q", test.in, got.Uris["Image"].Uri, test.want)
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
			var got albumImage
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
			if got.Latitude.valid != test.wantHasLat {
				t.Fatalf("Latitude present = %v, want %v", got.Latitude.valid, test.wantHasLat)
			}
			if got.Latitude.valid && got.Latitude.value != test.wantLatitude {
				t.Fatalf("Latitude = %v, want %v", got.Latitude.value, test.wantLatitude)
			}
			if got.Longitude.valid != test.wantHasLong {
				t.Fatalf("Longitude present = %v, want %v", got.Longitude.valid, test.wantHasLong)
			}
			if got.Longitude.valid && got.Longitude.value != test.wantLongitude {
				t.Fatalf("Longitude = %v, want %v", got.Longitude.value, test.wantLongitude)
			}
			if got.Altitude.valid != test.wantHasAlt {
				t.Fatalf("Altitude present = %v, want %v", got.Altitude.valid, test.wantHasAlt)
			}
			if got.Altitude.valid && got.Altitude.value != test.wantAltitude {
				t.Fatalf("Altitude = %v, want %v", got.Altitude.value, test.wantAltitude)
			}
		})
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
