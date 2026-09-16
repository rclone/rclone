package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyRelativePath(t *testing.T) {
	for _, test := range []struct {
		name          string
		rootDirectory string
		key           string
		wantPath      string
		wantOK        bool
	}{
		{"root empty", "", "a/b.txt", "a/b.txt", true},
		{"under prefix", "docs", "docs/a/b.txt", "a/b.txt", true},
		{"sibling sharing prefix", "docs", "docsets/x", "", false},
		{"directory marker of root", "docs", "docs/", "", true},
		{"key equals root", "docs", "docs", "", true},
		{"outside prefix", "docs", "other/x", "", false},
		{"nested root", "a/b", "a/b/c.txt", "c.txt", true},
		{"nested root near miss", "a/b", "a/bc.txt", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotPath, gotOK := notifyRelativePath(test.rootDirectory, test.key)
			assert.Equal(t, test.wantOK, gotOK)
			assert.Equal(t, test.wantPath, gotPath)
		})
	}
}

func TestNotifyEntryType(t *testing.T) {
	assert.Equal(t, fs.EntryObject, notifyEntryType("a/b.txt"))
	assert.Equal(t, fs.EntryDirectory, notifyEntryType("a/b/"))
	assert.Equal(t, fs.EntryObject, notifyEntryType(""))
}

func TestNotifyEventWanted(t *testing.T) {
	for _, test := range []struct {
		eventName string
		want      bool
	}{
		{"s3:ObjectCreated:Put", true},
		{"s3:ObjectCreated:CompleteMultipartUpload", true},
		{"s3:ObjectRemoved:Delete", true},
		{"s3:ObjectRemoved:DeleteMarkerCreated", true},
		{"s3:ObjectAccessed:Get", false},
		{"s3:Replication:OperationFailedReplication", false},
		{"", false},
		{"nonsense", false},
	} {
		assert.Equal(t, test.want, notifyEventWanted(test.eventName), test.eventName)
	}
}

func TestNotifyEndpointUnsupported(t *testing.T) {
	for _, test := range []struct {
		endpoint string
		want     bool
	}{
		{"https://s3.amazonaws.com", true},
		{"https://s3.us-east-1.amazonaws.com", true},
		{"https://storage.googleapis.com", true},
		{"https://minio.example.com", false},
		{"http://127.0.0.1:9000", false},
		{"https://notamazonaws.com", false},
	} {
		assert.Equal(t, test.want, notifyEndpointUnsupported(test.endpoint), test.endpoint)
	}
}

func TestNotifyURL(t *testing.T) {
	for _, test := range []struct {
		name           string
		endpoint       string
		bucket         string
		prefix         string
		forcePathStyle bool
		want           string
	}{
		{
			name: "path style with prefix", endpoint: "http://127.0.0.1:9000",
			bucket: "user-abc", prefix: "docs/", forcePathStyle: true,
			want: "http://127.0.0.1:9000/user-abc?events=s3%3AObjectCreated%3A%2A&events=s3%3AObjectRemoved%3A%2A&ping=10&prefix=docs%2F&suffix=",
		},
		{
			name: "path style no prefix", endpoint: "http://127.0.0.1:9000",
			bucket: "user-abc", prefix: "", forcePathStyle: true,
			want: "http://127.0.0.1:9000/user-abc?events=s3%3AObjectCreated%3A%2A&events=s3%3AObjectRemoved%3A%2A&ping=10&prefix=&suffix=",
		},
		{
			name: "virtual host style", endpoint: "https://minio.example.com",
			bucket: "user-abc", prefix: "", forcePathStyle: false,
			want: "https://user-abc.minio.example.com/?events=s3%3AObjectCreated%3A%2A&events=s3%3AObjectRemoved%3A%2A&ping=10&prefix=&suffix=",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := notifyURL(test.endpoint, test.bucket, test.prefix, test.forcePathStyle)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)

			// the two events parameters must survive in the order given
			u, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"}, u.Query()["events"])
			assert.Equal(t, "10", u.Query().Get("ping"))
		})
	}
}

func TestNotifyURLBadEndpoint(t *testing.T) {
	_, err := notifyURL("://not a url", "bucket", "", true)
	assert.Error(t, err)
	_, err = notifyURL("noscheme", "bucket", "", true)
	assert.Error(t, err)
}

// notifyRecorder records the calls made to a notifyFunc.
type notifyRecorder struct {
	mu    sync.Mutex
	calls []notifyCall
}

type notifyCall struct {
	path      string
	entryType fs.EntryType
}

func (r *notifyRecorder) fn(path string, entryType fs.EntryType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, notifyCall{path, entryType})
}

// events returns the recorded calls with the root invalidations, which every
// successful connection emits, removed.
func (r *notifyRecorder) events() []notifyCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []notifyCall{}
	for _, call := range r.calls {
		if call.path == "" && call.entryType == fs.EntryDirectory {
			continue
		}
		out = append(out, call)
	}
	return out
}

func (r *notifyRecorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// testNotifyFs builds an Fs pointing at endpoint, with anonymous credentials
// so that signing succeeds without any configuration.
func testNotifyFs(t *testing.T, endpoint, bucket, rootDirectory string) *Fs {
	t.Helper()
	return &Fs{
		name:          "test",
		rootBucket:    bucket,
		rootDirectory: rootDirectory,
		srv:           http.DefaultClient,
		c: s3.New(s3.Options{
			Region:      "us-east-1",
			Credentials: credentialsProvider{},
		}),
		opt: Options{
			Provider:       "Minio",
			Endpoint:       endpoint,
			Region:         "us-east-1",
			ForcePathStyle: true,
		},
	}
}

// credentialsProvider hands out fixed credentials for signing in tests.
type credentialsProvider struct{}

func (credentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: "KEY", SecretAccessKey: "SECRET"}, nil
}

// notifyTestServer serves the given lines as a notification stream, recording
// how many requests it has received.
func notifyTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// streamLines writes lines to w, flushing each one so the client sees them
// as they are produced.
func streamLines(w http.ResponseWriter, lines ...string) {
	flusher, _ := w.(http.Flusher)
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

const testEvent = `{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"bucket":{"name":"user-abc"},"object":{"key":"docs/report.txt"}}}]}`

func TestListenOnceEvent(t *testing.T) {
	server, requests := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/user-abc", r.URL.Path)
		assert.Equal(t, "docs/", r.URL.Query().Get("prefix"))
		assert.Equal(t, emptySHA256Hex, r.Header.Get("X-Amz-Content-Sha256"))
		assert.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
		streamLines(w, testEvent)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "docs")
	recorder := &notifyRecorder{}
	delivered, err := f.listenOnce(context.Background(), recorder.fn)
	require.NoError(t, err)
	assert.True(t, delivered)
	assert.Equal(t, int32(1), atomic.LoadInt32(requests))
	assert.Equal(t, []notifyCall{{"report.txt", fs.EntryObject}}, recorder.events())
}

func TestListenOnceKeepaliveAndMalformed(t *testing.T) {
	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// keepalives in both the shapes MinIO sends, then a malformed line,
		// then a real event: the stream must survive the bad line
		streamLines(w, `{"Records":null}`, `{"Records":[]}`, `{not json`, testEvent)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "docs")
	recorder := &notifyRecorder{}
	delivered, err := f.listenOnce(context.Background(), recorder.fn)
	require.NoError(t, err)
	assert.True(t, delivered)
	assert.Equal(t, []notifyCall{{"report.txt", fs.EntryObject}}, recorder.events())
}

func TestListenOnceMultiRecord(t *testing.T) {
	line := `{"Records":[` +
		`{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"docs/a.txt"}}},` +
		`{"eventName":"s3:ObjectRemoved:Delete","s3":{"object":{"key":"docs/b.txt"}}},` +
		`{"eventName":"s3:ObjectAccessed:Get","s3":{"object":{"key":"docs/c.txt"}}},` +
		`{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"other/d.txt"}}},` +
		`{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"docs/sub/"}}}` +
		`]}`
	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		streamLines(w, line)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "docs")
	recorder := &notifyRecorder{}
	_, err := f.listenOnce(context.Background(), recorder.fn)
	require.NoError(t, err)
	// the accessed event and the key outside the prefix are both dropped
	assert.Equal(t, []notifyCall{
		{"a.txt", fs.EntryObject},
		{"b.txt", fs.EntryObject},
		{"sub/", fs.EntryDirectory},
	}, recorder.events())
}

// TestListenOnceKeysAreVerbatim pins the key encoding. MinIO sends object keys
// unescaped, so these names must arrive byte for byte - decoding them would
// turn "plus+file.txt" into "plus file.txt". Verified against a live MinIO and
// against minio-go, which does not unescape either.
func TestListenOnceKeysAreVerbatim(t *testing.T) {
	names := []string{
		"plus+file.txt",
		"space file.txt",
		"percent%20literal.txt",
		"amp&and=eq.txt",
		"unicode-caf\u00e9.txt",
	}
	line := `{"Records":[`
	for i, name := range names {
		if i > 0 {
			line += ","
		}
		line += `{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"docs/` + name + `"}}}`
	}
	line += `]}`

	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		streamLines(w, line)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "docs")
	recorder := &notifyRecorder{}
	_, err := f.listenOnce(context.Background(), recorder.fn)
	require.NoError(t, err)

	want := []notifyCall{}
	for _, name := range names {
		want = append(want, notifyCall{name, fs.EntryObject})
	}
	assert.Equal(t, want, recorder.events())
}

func TestListenOnceRootInvalidationOnConnect(t *testing.T) {
	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		streamLines(w, `{"Records":null}`)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "")
	recorder := &notifyRecorder{}
	_, err := f.listenOnce(context.Background(), recorder.fn)
	require.NoError(t, err)
	// a successful connection always invalidates the root, since events
	// missed while disconnected cannot be enumerated
	assert.Equal(t, []notifyCall{{"", fs.EntryDirectory}}, recorder.calls)
}

func TestListenOncePermanentFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		notice bool
	}{
		{"not implemented", http.StatusNotImplemented, "", false},
		{"not implemented code", http.StatusBadRequest, "<Error><Code>NotImplemented</Code></Error>", false},
		{"access denied", http.StatusForbidden, "", true},
		{"access denied code", http.StatusBadRequest, "<Error><Code>AccessDenied</Code></Error>", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			})
			f := testNotifyFs(t, server.URL, "user-abc", "")
			recorder := &notifyRecorder{}
			_, err := f.listenOnce(context.Background(), recorder.fn)
			require.Error(t, err)
			var permanent errPermanent
			require.ErrorAs(t, err, &permanent)
			assert.Equal(t, test.notice, permanent.notice)
			assert.Empty(t, recorder.calls)
		})
	}
}

func TestListenOnceRetryableFailure(t *testing.T) {
	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := testNotifyFs(t, server.URL, "user-abc", "")
	_, err := f.listenOnce(context.Background(), (&notifyRecorder{}).fn)
	require.Error(t, err)
	var permanent errPermanent
	assert.False(t, errors.As(err, &permanent), "5xx must be retryable")
}

func TestListenOnceUnsupportedEndpoint(t *testing.T) {
	f := testNotifyFs(t, "https://s3.amazonaws.com", "user-abc", "")
	_, err := f.listenOnce(context.Background(), (&notifyRecorder{}).fn)
	require.Error(t, err)
	var permanent errPermanent
	assert.True(t, errors.As(err, &permanent))

	f = testNotifyFs(t, "", "user-abc", "")
	_, err = f.listenOnce(context.Background(), (&notifyRecorder{}).fn)
	require.Error(t, err)
	assert.True(t, errors.As(err, &permanent))
}

func TestListenBucketNotificationReconnects(t *testing.T) {
	server, requests := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// serve one event then drop the connection, forcing a reconnect
		streamLines(w, testEvent)
	})

	f := testNotifyFs(t, server.URL, "user-abc", "docs")
	recorder := &notifyRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	var disabled atomic.Bool
	go func() {
		defer close(done)
		f.listenBucketNotification(ctx, recorder.fn, &disabled)
	}()

	// wait for at least two connections, which means a reconnect happened
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(requests) >= 2
	}, 30*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("listenBucketNotification did not return after cancel")
	}

	assert.False(t, disabled.Load())
	// each connection emits a root invalidation as well as the event
	var roots, events int
	recorder.mu.Lock()
	for _, call := range recorder.calls {
		if call.path == "" && call.entryType == fs.EntryDirectory {
			roots++
		} else {
			events++
		}
	}
	recorder.mu.Unlock()
	assert.GreaterOrEqual(t, roots, 2, "each reconnect must invalidate the root")
	assert.GreaterOrEqual(t, events, 2)
}

func TestListenBucketNotificationStopsPermanently(t *testing.T) {
	for _, status := range []int{http.StatusNotImplemented, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server, requests := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			})
			f := testNotifyFs(t, server.URL, "user-abc", "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			var disabled atomic.Bool
			go func() {
				defer close(done)
				f.listenBucketNotification(ctx, (&notifyRecorder{}).fn, &disabled)
			}()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("listenBucketNotification did not give up")
			}
			assert.True(t, disabled.Load())

			// well past the minimum backoff, so a retry would have shown up
			time.Sleep(2 * notifyMinBackoff)
			assert.Equal(t, int32(1), atomic.LoadInt32(requests), "must not reconnect")
		})
	}
}

func TestChangeNotifyLifecycle(t *testing.T) {
	server, requests := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		streamLines(w, `{"Records":null}`)
		// hold the connection open until the client goes away
		<-r.Context().Done()
	})

	f := testNotifyFs(t, server.URL, "user-abc", "")
	recorder := &notifyRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pollChan := make(chan time.Duration)
	f.changeNotify(ctx, recorder.fn, pollChan)

	// a non-zero interval starts a subscription
	pollChan <- time.Second
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(requests) == 1
	}, 10*time.Second, 10*time.Millisecond)

	// 0 stops it, and no new connection is made
	pollChan <- 0
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(requests))

	// a second non-zero value restarts it
	pollChan <- time.Second
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(requests) == 2
	}, 10*time.Second, 10*time.Millisecond)

	// closing the channel shuts everything down
	close(pollChan)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(2), atomic.LoadInt32(requests))
	assert.NotZero(t, recorder.len())
}

func TestChangeNotifyContextCancel(t *testing.T) {
	server, _ := notifyTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		streamLines(w, `{"Records":null}`)
		<-r.Context().Done()
	})

	f := testNotifyFs(t, server.URL, "user-abc", "")
	ctx, cancel := context.WithCancel(context.Background())
	before := runtime.NumGoroutine()

	pollChan := make(chan time.Duration, 1)
	f.changeNotify(ctx, (&notifyRecorder{}).fn, pollChan)
	pollChan <- time.Second
	time.Sleep(200 * time.Millisecond)
	cancel()

	// the goroutines must unwind rather than linger
	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= before+1
	}, 10*time.Second, 50*time.Millisecond)
}

func TestQuirkBucketNotificationsWiring(t *testing.T) {
	for _, test := range []struct {
		provider  string
		userSet   *fs.Tristate
		wantValue bool
	}{
		{provider: "Minio", wantValue: true},
		{provider: "AWS", wantValue: false},
		{provider: "Ceph", wantValue: false},
		{provider: "Other", wantValue: false},
		// an explicit false from the user must beat the Minio quirk
		{provider: "Minio", userSet: &fs.Tristate{Valid: true, Value: false}, wantValue: false},
		{provider: "AWS", userSet: &fs.Tristate{Valid: true, Value: true}, wantValue: true},
	} {
		name := test.provider
		if test.userSet != nil {
			name += "-explicit"
		}
		t.Run(name, func(t *testing.T) {
			provider := loadProvider(test.provider)
			require.NotNil(t, provider)
			opt := &Options{Provider: test.provider}
			if test.userSet != nil {
				opt.UseBucketNotifications = *test.userSet
			}
			setQuirks(opt, provider)
			assert.Equal(t, test.wantValue, opt.UseBucketNotifications.Value)
			assert.True(t, opt.UseBucketNotifications.Valid)
		})
	}
}
