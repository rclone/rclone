package seafile

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanUploadModTime(t *testing.T) {
	for version, want := range map[string]bool{
		"7.1.5":   false,
		"11.0.13": false,
		"12.0.3":  false,
		"12.0.4":  true,
		"12.0.14": true,
		"13.0.28": true,
	} {
		assert.Equal(t, want, canUploadModTime(semver.New(version)), version)
	}
}

func TestPrecision(t *testing.T) {
	assert.Equal(t, time.Second, (&Fs{uploadModTime: true}).Precision())
	assert.Equal(t, fs.ModTimeNotSupported, (&Fs{uploadModTime: false}).Precision())
}

func TestUploadSendsModTime(t *testing.T) {
	modTime := time.Date(2020, 1, 2, 3, 4, 5, 600, time.FixedZone("CET", 3600))
	for _, test := range []struct {
		name          string
		uploadModTime bool
		modTime       time.Time
		want          string
	}{
		{name: "supported", uploadModTime: true, modTime: modTime, want: "2020-01-02T02:04:05Z"},
		{name: "unsupported", uploadModTime: false, modTime: modTime},
		{name: "zero time", uploadModTime: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var (
				got     []string
				content string
			)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "1", r.URL.Query().Get("ret-json"))
				if !assert.NoError(t, r.ParseMultipartForm(1<<20)) {
					return
				}
				got = r.MultipartForm.Value["last_modify"]
				file, _, err := r.FormFile("file")
				if assert.NoError(t, err) {
					data, err := io.ReadAll(file)
					assert.NoError(t, err)
					content = string(data)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"name":"file.txt","size":5}]`))
			}))
			defer ts.Close()

			ctx := context.Background()
			f := &Fs{
				opt:           Options{Enc: encoder.EncodeSlash},
				srv:           rest.NewClient(ts.Client()),
				pacer:         getPacer(ctx, ts.URL),
				uploadModTime: test.uploadModTime,
			}
			_, err := f.upload(ctx, strings.NewReader("hello"), ts.URL+"/upload-api/token", "dir/file.txt", test.modTime)
			require.NoError(t, err)
			assert.Equal(t, "hello", content)
			if test.want == "" {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, []string{test.want}, got)
			}
		})
	}
}
