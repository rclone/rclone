//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

type testDispatcher func(req *http.Request) (*http.Response, error)

func (fn testDispatcher) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: req,
	}
}

func newTestListStartFs(ctx context.Context, t *testing.T, dispatcher common.HTTPRequestDispatcher, listStart string) *Fs {
	t.Helper()

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(&noAuthConfigurator{})
	if err != nil {
		t.Fatalf("NewObjectStorageClientWithConfigurationProvider() error = %v", err)
	}
	client.Host = "https://objectstorage.example.invalid"
	modifyClient(ctx, &Options{Provider: noAuth}, &client.BaseClient)
	client.HTTPClient = dispatcher

	return &Fs{
		name:       "test-oos",
		rootBucket: "test-bucket",
		opt: Options{
			Provider:  noAuth,
			Namespace: "test-namespace",
			Enc: encoder.EncodeInvalidUtf8 |
				encoder.EncodeSlash |
				encoder.EncodeDot,
			ListStart: listStart,
		},
		srv:   &client,
		pacer: fs.NewPacer(ctx, pacer.NewDefault()),
		cache: bucket.NewCache(),
	}
}

func TestListStartExistsEmptyStart(t *testing.T) {
	f := &Fs{}

	exists, err := f.listStartExists(t.Context(), "bucket-a", "")
	if err != nil {
		t.Fatalf("listStartExists() error = %v", err)
	}
	if exists {
		t.Fatalf("listStartExists() exists = true, want false")
	}
}

func TestListStartExistsNotFound(t *testing.T) {
	ctx := t.Context()

	var headCalls int
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o/missing-start"):
			headCalls++
			return newTestResponse(r, http.StatusNotFound, `{"code":"ObjectNotFound","message":"missing object"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "missing-start")
	exists, err := f.listStartExists(ctx, "test-bucket", "missing-start")
	if err != nil {
		t.Fatalf("listStartExists() error = %v", err)
	}
	if exists {
		t.Fatalf("listStartExists() exists = true, want false")
	}
	if headCalls != 1 {
		t.Fatalf("HeadObject call count = %d, want 1", headCalls)
	}
}

func TestListStartMissingReturnsStructuredError(t *testing.T) {
	ctx := t.Context()

	var headCalls int
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o/missing-start"):
			headCalls++
			return newTestResponse(r, http.StatusNotFound, `{"code":"ObjectNotFound","message":"missing object"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "missing-start")

	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		return nil
	})
	if err == nil {
		t.Fatal("list() error = nil, want structured error")
	}
	if headCalls != 1 {
		t.Fatalf("HeadObject call count = %d, want 1", headCalls)
	}
	if !strings.Contains(err.Error(), `"code":"list_start_not_found"`) {
		t.Fatalf("list() error = %q, want list_start_not_found", err)
	}
	if !strings.Contains(err.Error(), `"option":"--oos-list-start"`) {
		t.Fatalf("list() error = %q, want option metadata", err)
	}
}

func TestListStartCheckpointTakesPrecedence(t *testing.T) {
	ctx := t.Context()

	checkpointPath := t.TempDir() + "/checkpoint.json"
	if err := saveListingCheckpoint(checkpointPath, listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "test-bucket",
		Prefix:    "",
		Delimiter: "",
		Marker:    "checkpoint-start",
	}); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	var (
		headCalls  int
		listStarts []string
	)
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o/checkpoint-start"):
			headCalls++
			return newTestResponse(r, http.StatusOK, ""), nil
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o"):
			listStarts = append(listStarts, r.URL.Query().Get("start"))
			return newTestResponse(r, http.StatusOK, `{"objects":[{"name":"checkpointed.txt","size":1,"etag":"etag-1"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "missing-start")
	f.opt.ListCheckpointFile = checkpointPath

	var got []string
	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		if !isDirectory {
			got = append(got, remote)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list() error = %v", err)
	}
	if want := []string{"checkpointed.txt"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listed objects = %v, want %v", got, want)
	}
	if headCalls != 1 {
		t.Fatalf("HeadObject call count = %d, want 1", headCalls)
	}
	if len(listStarts) != 1 {
		t.Fatalf("ListObjects call count = %d, want 1", len(listStarts))
	}
	if listStarts[0] != "checkpoint-start" {
		t.Fatalf("ListObjects start query = %q, want %q", listStarts[0], "checkpoint-start")
	}
}

func TestStaleCheckpointMarkerReturnsStructuredError(t *testing.T) {
	ctx := t.Context()

	checkpointPath := t.TempDir() + "/checkpoint.json"
	if err := saveListingCheckpoint(checkpointPath, listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "test-bucket",
		Prefix:    "",
		Delimiter: "",
		Marker:    "stale-marker",
	}); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	var headCalls int
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o/stale-marker"):
			headCalls++
			return newTestResponse(r, http.StatusNotFound, `{"code":"ObjectNotFound","message":"missing object"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "")
	f.opt.ListCheckpointFile = checkpointPath

	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		return nil
	})
	if err == nil {
		t.Fatal("list() error = nil, want structured error")
	}
	if headCalls != 1 {
		t.Fatalf("HeadObject call count = %d, want 1", headCalls)
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_marker_not_found"`) {
		t.Fatalf("list() error = %q, want checkpoint_marker_not_found", err)
	}
	if !strings.Contains(err.Error(), `"option":"--oos-list-checkpoint-file"`) {
		t.Fatalf("list() error = %q, want checkpoint option metadata", err)
	}
}

func TestListStartEmptyCheckpointTakesPrecedence(t *testing.T) {
	ctx := t.Context()

	checkpointPath := t.TempDir() + "/checkpoint.json"
	if err := saveListingCheckpoint(checkpointPath, listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "test-bucket",
		Prefix:    "",
		Delimiter: "",
		Marker:    "",
	}); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	var (
		headCalls  int
		listStarts []string
	)
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead:
			headCalls++
			return newTestResponse(r, http.StatusNotFound, `{"code":"ObjectNotFound","message":"missing object"}`), nil
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o"):
			listStarts = append(listStarts, r.URL.Query().Get("start"))
			return newTestResponse(r, http.StatusOK, `{"objects":[{"name":"from-beginning.txt","size":1,"etag":"etag-1"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "missing-start")
	f.opt.ListCheckpointFile = checkpointPath

	var got []string
	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		if !isDirectory {
			got = append(got, remote)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list() error = %v", err)
	}
	if want := []string{"from-beginning.txt"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listed objects = %v, want %v", got, want)
	}
	if headCalls != 0 {
		t.Fatalf("HeadObject call count = %d, want 0", headCalls)
	}
	if len(listStarts) != 1 {
		t.Fatalf("ListObjects call count = %d, want 1", len(listStarts))
	}
	if listStarts[0] != "" {
		t.Fatalf("ListObjects start query = %q, want empty", listStarts[0])
	}
}

func TestListStartCheckpointInvalidForScopeReturnsStructuredError(t *testing.T) {
	ctx := t.Context()

	checkpointPath := t.TempDir() + "/checkpoint.json"
	if err := saveListingCheckpoint(checkpointPath, listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "test-bucket",
		Prefix:    "other-prefix/",
		Delimiter: "",
		Marker:    "checkpoint-start",
	}); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	f := newTestListStartFs(ctx, t, testDispatcher(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		return nil, nil
	}), "")
	f.opt.ListCheckpointFile = checkpointPath

	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		return nil
	})
	if err == nil {
		t.Fatal("list() error = nil, want structured error")
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_scope_mismatch"`) {
		t.Fatalf("list() error = %q, want checkpoint_scope_mismatch", err)
	}
}

func TestListStartExistingUsesMarker(t *testing.T) {
	ctx := t.Context()

	var (
		headCalls  int
		listStarts []string
	)
	dispatcher := testDispatcher(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o/existing-start"):
			headCalls++
			return newTestResponse(r, http.StatusOK, ""), nil
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/n/test-namespace/b/test-bucket/o"):
			listStarts = append(listStarts, r.URL.Query().Get("start"))
			return newTestResponse(r, http.StatusOK, `{"objects":[{"name":"omega.txt","size":1,"etag":"etag-1"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	f := newTestListStartFs(ctx, t, dispatcher, "existing-start")

	var got []string
	err := f.list(ctx, "test-bucket", "", "", false, true, 0, func(remote string, _ *objectstorage.ObjectSummary, isDirectory bool) error {
		if !isDirectory {
			got = append(got, remote)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list() error = %v", err)
	}
	if want := []string{"omega.txt"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listed objects = %v, want %v", got, want)
	}
	if headCalls != 1 {
		t.Fatalf("HeadObject call count = %d, want 1", headCalls)
	}
	if len(listStarts) != 1 {
		t.Fatalf("ListObjects call count = %d, want 1", len(listStarts))
	}
	if listStarts[0] != "existing-start" {
		t.Fatalf("ListObjects start query = %q, want %q", listStarts[0], "existing-start")
	}
}
