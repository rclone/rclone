package linkbox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// apiRedirect sends requests for the Linkbox web API to the test server.
type apiRedirect struct {
	target *url.URL
	base   http.RoundTripper
}

// RoundTrip rewrites the destination host so the real API URL can be used.
func (t apiRedirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return t.base.RoundTrip(req)
}

// newTestFs returns an Fs whose web API requests go to handler.
func newTestFs(t *testing.T, handler http.HandlerFunc) *Fs {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &Fs{
		name:     "test",
		srv:      rest.NewClient(&http.Client{Transport: apiRedirect{target: target, base: server.Client().Transport}}),
		pacer:    fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(0))),
		webToken: "tok",
	}
}

func TestAbout(t *testing.T) {
	f := newTestFs(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/user/info" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.Query().Get("token"); got != "tok" {
			t.Errorf("token = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":1,"data":{"size_cap":1099,"size_curr":249}}`)
	})

	usage, err := f.About(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if usage.Total == nil || usage.Used == nil || usage.Free == nil ||
		*usage.Total != 1099 || *usage.Used != 249 || *usage.Free != 850 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestAboutBadQuota(t *testing.T) {
	f := newTestFs(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":1,"data":{"size_cap":100,"size_curr":200}}`)
	})

	if _, err := f.About(context.Background()); err == nil {
		t.Fatal("expected an error for used > total")
	}
}

func TestAboutAPIError(t *testing.T) {
	f := newTestFs(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":0,"msg":"ip incorrect"}`)
	})

	if _, err := f.About(context.Background()); err == nil {
		t.Fatal("expected an error for a failed API call")
	}
}
