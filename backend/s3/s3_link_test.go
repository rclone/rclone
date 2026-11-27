package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLinkTestFs(t *testing.T, observers ...func(*http.Request)) *Fs {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, observe := range observers {
			observe(r)
		}
		if r.URL.Query().Has("versions") {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "prefix/file", r.URL.Query().Get("prefix"))
			_, err := io.WriteString(w, `<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
<IsTruncated>false</IsTruncated>
<Version><Key>prefix/file</Key><VersionId>version+id/1</VersionId><IsLatest>false</IsLatest>
<LastModified>2024-01-01T00:00:00Z</LastModified><Size>1</Size></Version>
</ListVersionsResult>`)
			assert.NoError(t, err)
			return
		}
		assert.Equal(t, http.MethodHead, r.Method)
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", "1")
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
	}))
	t.Cleanup(server.Close)

	ctx, opt, client := SetupS3Test(t)
	ctx, ci := fs.AddConfig(ctx)
	ci.LowLevelRetries = 1
	opt.Endpoint = server.URL
	opt.ForcePathStyle = true
	opt.Region = "us-east-1"
	opt.AccessKeyID = "test-access-key"
	opt.SecretAccessKey = "test-secret-key"
	opt.SessionToken = "test-token+/="
	c, _, err := s3Connection(ctx, opt, client)
	require.NoError(t, err)
	f := &Fs{
		name:                "s3test",
		opt:                 *opt,
		ctx:                 ctx,
		c:                   c,
		pacer:               fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep))),
		cache:               bucket.NewCache(),
		urlFetchLogSafe:     true,
		urlFetchRequestSafe: true,
	}
	f.setRoot("bucket/prefix")
	return f
}

func linkSignature(t *testing.T, u *url.URL) string {
	t.Helper()
	q := u.Query()
	signingTime, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	require.NoError(t, err)
	q.Del("X-Amz-Signature")
	unsigned := *u
	unsigned.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, unsigned.String(), nil)
	require.NoError(t, err)
	signed, _, err := v4.NewSigner().PresignHTTP(context.Background(), aws.Credentials{
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		SessionToken:    "test-token+/=",
	}, req, "UNSIGNED-PAYLOAD", "s3", "us-east-1", signingTime, func(opt *v4.SignerOptions) {
		opt.DisableURIPathEscaping = true
	})
	require.NoError(t, err)
	result, err := url.Parse(signed)
	require.NoError(t, err)
	return result.Query().Get("X-Amz-Signature")
}

func TestCommandLink(t *testing.T) {
	f := newLinkTestFs(t)
	remote := "dir/a file +&?%\u2603.txt"
	overrides := map[string]string{
		"response-cache-control":       "private, max-age=0",
		"response-content-disposition": `attachment; filename="a +&?.txt"`,
		"response-content-encoding":    "identity",
		"response-content-language":    "en-US",
		"response-content-type":        "text/plain; charset=utf-8",
		"response-expires":             "Thu, 01 Jan 1970 00:00:00 GMT",
	}
	opts := map[string]string{"expire": "1h"}
	for k, v := range overrides {
		opts[k] = v
	}
	out, err := f.Command(context.Background(), "link", []string{remote}, opts)
	require.NoError(t, err)
	link, ok := out.(string)
	require.True(t, ok)
	u, err := url.Parse(link)
	require.NoError(t, err)
	assert.Equal(t, "/bucket/prefix/"+remote, u.Path)
	assert.NotContains(t, u.RawQuery, " ")
	q := u.Query()
	assert.Equal(t, "3600", q.Get("X-Amz-Expires"))
	assert.Equal(t, "test-token+/=", q.Get("X-Amz-Security-Token"))
	assert.Equal(t, "AWS4-HMAC-SHA256", q.Get("X-Amz-Algorithm"))
	assert.Equal(t, "host", q.Get("X-Amz-SignedHeaders"))
	for k, v := range overrides {
		assert.Equal(t, v, q.Get(k), k)
	}
	signature := q.Get("X-Amz-Signature")
	require.NotEmpty(t, signature)
	assert.Equal(t, signature, linkSignature(t, u))
	for k := range overrides {
		t.Run("Signed/"+k, func(t *testing.T) {
			tampered := *u
			changed := u.Query()
			changed.Set(k, "changed")
			tampered.RawQuery = changed.Encode()
			assert.NotEqual(t, signature, linkSignature(t, &tampered))
		})
	}
}

func TestCommandLinkExpire(t *testing.T) {
	f := newLinkTestFs(t)
	for _, test := range []struct {
		name   string
		opts   map[string]string
		expire string
	}{
		{"Default", nil, "604800"},
		{"Minimum", map[string]string{"expire": "1s"}, "1"},
		{"Maximum", map[string]string{"expire": "7d"}, "604800"},
		{"Clamped", map[string]string{"expire": "8d"}, "604800"},
		{"Off", map[string]string{"expire": "off"}, "604800"},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := f.Command(context.Background(), "link", []string{"file"}, test.opts)
			require.NoError(t, err)
			u, err := url.Parse(out.(string))
			require.NoError(t, err)
			assert.Equal(t, test.expire, u.Query().Get("X-Amz-Expires"))
			for key := range u.Query() {
				assert.False(t, strings.HasPrefix(key, "response-"))
			}
			assert.Equal(t, u.Query().Get("X-Amz-Signature"), linkSignature(t, u))
		})
	}
}

func TestCommandLinkErrors(t *testing.T) {
	f := newLinkTestFs(t)
	for _, test := range []struct {
		name string
		args []string
		opts map[string]string
	}{
		{"NoPath", nil, nil},
		{"TooManyPaths", []string{"one", "two"}, nil},
		{"EmptyPath", []string{""}, nil},
		{"Directory", []string{"dir/"}, nil},
		{"Missing", []string{"missing"}, nil},
		{"UnknownOption", []string{"file"}, map[string]string{"response-unknown": "value"}},
		{"InvalidDate", []string{"file"}, map[string]string{"response-expires": "tomorrow"}},
		{"InvalidExpire", []string{"file"}, map[string]string{"expire": "invalid"}},
		{"EmptyExpire", []string{"file"}, map[string]string{"expire": ""}},
		{"ZeroExpire", []string{"file"}, map[string]string{"expire": "0"}},
		{"NegativeExpire", []string{"file"}, map[string]string{"expire": "-1s"}},
		{"SubsecondExpire", []string{"file"}, map[string]string{"expire": "500ms"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.Command(context.Background(), "link", test.args, test.opts)
			require.Error(t, err)
			assert.NotErrorIs(t, err, fs.ErrorCommandNotFound)
		})
	}
}

func TestCommandLinkVersionAt(t *testing.T) {
	f := newLinkTestFs(t)
	f.opt.VersionAt = fs.Time(time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC))
	for _, command := range []bool{false, true} {
		var link string
		if command {
			out, err := f.Command(context.Background(), "link", []string{"file"}, map[string]string{
				"response-content-type": "text/plain",
			})
			require.NoError(t, err)
			link = out.(string)
		} else {
			var err error
			link, err = f.PublicLink(context.Background(), "file", fs.DurationOff, false)
			require.NoError(t, err)
		}
		u, err := url.Parse(link)
		require.NoError(t, err)
		assert.Equal(t, "/bucket/prefix/file", u.Path)
		assert.Equal(t, "version+id/1", u.Query().Get("versionId"))
		assert.Equal(t, u.Query().Get("X-Amz-Signature"), linkSignature(t, u))
	}
}

func TestPublicLink(t *testing.T) {
	f := newLinkTestFs(t)
	link, err := f.PublicLink(context.Background(), "file", fs.DurationOff, false)
	require.NoError(t, err)
	u, err := url.Parse(link)
	require.NoError(t, err)
	assert.Equal(t, "/bucket/prefix/file", u.Path)
	assert.Equal(t, "604800", u.Query().Get("X-Amz-Expires"))
	assert.Equal(t, u.Query().Get("X-Amz-Signature"), linkSignature(t, u))
	_, err = f.PublicLink(context.Background(), "dir/", fs.DurationOff, false)
	assert.ErrorIs(t, err, fs.ErrorCantShareDirectories)
	_, err = f.PublicLink(context.Background(), "missing", fs.DurationOff, false)
	assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

func TestPublicLinkRequestFixups(t *testing.T) {
	for _, tt := range []struct {
		name               string
		useXID             bool
		signAcceptEncoding bool
	}{
		{"no x-id", false, true},
		{"unsigned Accept-Encoding", true, false},
		{"no x-id and unsigned Accept-Encoding", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newLinkTestFs(t)
			f.opt.UseXID = fs.Tristate{Value: tt.useXID, Valid: true}
			f.opt.SignAcceptEncoding = fs.Tristate{Value: tt.signAcceptEncoding, Valid: true}
			c, _, err := s3Connection(f.ctx, &f.opt, getClient(f.ctx, &f.opt))
			require.NoError(t, err)
			f.c = c
			link, err := f.PublicLink(context.Background(), "file", fs.Duration(time.Hour), false)
			require.NoError(t, err)
			u, err := url.Parse(link)
			require.NoError(t, err)
			assert.Equal(t, tt.useXID, u.Query().Has("x-id"))
			assert.Equal(t, "host", u.Query().Get("X-Amz-SignedHeaders"))
			assert.Equal(t, u.Query().Get("X-Amz-Signature"), linkSignature(t, u))
		})
	}
}

func TestDirectPublicLinkPresigning(t *testing.T) {
	for _, tt := range []struct {
		name   string
		expire time.Duration
		want   string
	}{
		{"one second", time.Second, "1"},
		{"seventeen minutes", 17 * time.Minute, "1020"},
		{"one hour", time.Hour, "3600"},
		{"one day", 24 * time.Hour, "86400"},
		{"seven days", 7 * 24 * time.Hour, "604800"},
		{"clamped eight days", 8 * 24 * time.Hour, "604800"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var heads, gets atomic.Int32
			f := newLinkTestFs(t, func(r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					heads.Add(1)
				case http.MethodGet:
					gets.Add(1)
				}
			})
			require.True(t, f.supportsDirectPublicLink())
			remote := "dir/a file +&?%\u2603.txt"
			link, err := f.PublicLink(context.Background(), remote, fs.Duration(tt.expire), false)
			require.NoError(t, err)
			u, err := url.Parse(link)
			require.NoError(t, err)
			assert.Equal(t, "/bucket/prefix/"+remote, u.Path)
			assert.Equal(t, "/bucket/prefix/dir/a%20file%20%2B%26%3F%25%E2%98%83.txt", u.EscapedPath())
			q := u.Query()
			assert.Equal(t, tt.want, q.Get("X-Amz-Expires"))
			assert.Equal(t, "host", q.Get("X-Amz-SignedHeaders"))
			assert.Equal(t, "test-token+/=", q.Get("X-Amz-Security-Token"))
			assert.Contains(t, u.RawQuery, "X-Amz-Security-Token=test-token%2B%2F%3D")
			assert.NotContains(t, u.RawQuery, " ")
			require.NotEmpty(t, q.Get("X-Amz-Signature"))
			assert.Equal(t, q.Get("X-Amz-Signature"), linkSignature(t, u))
			assert.Equal(t, int32(1), heads.Load())
			assert.Zero(t, gets.Load(), "presigning must not download the source body")
		})
	}
}
