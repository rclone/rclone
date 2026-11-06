package imagekit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testContent = "0123456789"

func newOpenTestObject(t *testing.T, srv *httptest.Server, path string) *Object {
	ctx := context.Background()
	ik, err := client.New(ctx, client.NewParams{
		URLEndpoint: srv.URL,
		PublicKey:   "public",
		PrivateKey:  "private",
	})
	require.NoError(t, err)
	isPrivateFile := false
	return &Object{
		fs: &Fs{
			ik:  ik,
			srv: fshttp.NewClient(ctx),
		},
		remote: "file.txt",
		file: client.File{
			URL:           srv.URL + path,
			IsPrivateFile: &isPrivateFile,
			Size:          uint64(len(testContent)),
		},
	}
}

func TestOpenHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/file.txt":
			http.ServeContent(w, r, "file.txt", time.Time{}, strings.NewReader(testContent))
		case "/missing.txt":
			http.Error(w, "<html>Not Found</html>", http.StatusNotFound)
		default:
			http.Error(w, "<html>Bad Gateway</html>", http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	for _, test := range []struct {
		name    string
		path    string
		options []fs.OpenOption
		want    string
		wantErr string
	}{
		{name: "OK", path: "/file.txt", want: testContent},
		{name: "Range", path: "/file.txt", options: []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}}, want: "234"},
		{name: "NotFound", path: "/missing.txt", wantErr: "404 Not Found"},
		{name: "BadGateway", path: "/error.txt", wantErr: "502 Bad Gateway"},
	} {
		t.Run(test.name, func(t *testing.T) {
			o := newOpenTestObject(t, srv, test.path)
			in, err := o.Open(ctx, test.options...)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				assert.Nil(t, in)
				return
			}
			require.NoError(t, err)
			got, err := io.ReadAll(in)
			require.NoError(t, err)
			require.NoError(t, in.Close())
			assert.Equal(t, test.want, string(got))
		})
	}
}
