package drime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/drime/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPerPage = 5  // entries per page the test server returns
	testLimit   = 20 // entries the test server will page through
)

// listingServer serves /drive/file-entries from entries the way Drime
// does.
//
// It returns testPerPage entries per page and won't page past testLimit
// entries. Asking for a later page returns the last page it allows
// again, with last_page one beyond it.
//
// It counts the requests made in requests.
func listingServer(t *testing.T, entries []api.Item, requests *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assert.Equal(t, "/drive/file-entries", r.URL.Path)
		params := r.URL.Query()
		page, err := strconv.Atoi(params.Get("page"))
		require.NoError(t, err)
		maxPage := testLimit / testPerPage
		currentPage := min(page, maxPage)
		start := min((currentPage-1)*testPerPage, len(entries))
		end := min(start+testPerPage, len(entries))
		lastPage := currentPage
		if page > maxPage || end < len(entries) {
			lastPage = currentPage + 1
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"current_page": currentPage,
			"last_page":    lastPage,
			"per_page":     testPerPage,
			"total":        (currentPage + 1) * testPerPage,
			"data":         entries[start:end],
		}))
	}))
}

// makeEntries makes n file entries
func makeEntries(n int) []api.Item {
	entries := make([]api.Item, n)
	for i := range entries {
		entries[i] = api.Item{
			ID:   json.Number(strconv.Itoa(1000 + i)),
			Name: fmt.Sprintf("file%05d", i),
			Type: "text",
		}
	}
	return entries
}

// testFs makes an Fs which uses server as its API
func testFs(t *testing.T, server *httptest.Server) *Fs {
	t.Helper()
	f := &Fs{
		opt: Options{ListChunk: 1000},
		srv: rest.NewClient(server.Client()).SetRoot(server.URL),
	}
	f.pacer = fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	f.srv.SetErrorHandler(errorHandler)
	return f
}

// listNames lists a directory with listAll and returns the names found
func listNames(ctx context.Context, f *Fs) (names []string, err error) {
	_, err = f.listAll(ctx, "1", false, false, "", func(item *api.Item) bool {
		names = append(names, item.Name)
		return false
	})
	return names, err
}

// TestListAll checks that listAll returns every entry exactly once
func TestListAll(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries int
	}{
		{name: "Empty", entries: 0},
		{name: "OnePage", entries: 3},
		{name: "ExactPages", entries: 10},
		{name: "PartialLastPage", entries: 13},
		{name: "AtLimit", entries: testLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var requests atomic.Int64
			entries := makeEntries(test.entries)
			server := listingServer(t, entries, &requests)
			defer server.Close()
			f := testFs(t, server)

			names, err := listNames(ctx, f)
			require.NoError(t, err)
			var want []string
			for _, entry := range entries {
				want = append(want, entry.Name)
			}
			assert.ElementsMatch(t, want, names)
		})
	}
}

// TestListAllTooManyEntries checks that listAll returns an error for a
// directory with more entries than the server will list, rather than
// looping forever.
func TestListAllTooManyEntries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var requests atomic.Int64
	server := listingServer(t, makeEntries(2*testLimit), &requests)
	defer server.Close()
	f := testFs(t, server)

	_, err := listNames(ctx, f)
	require.Error(t, err)
	assert.NoError(t, ctx.Err(), "listing didn't finish")
	assert.LessOrEqual(t, requests.Load(), int64(testLimit/testPerPage+1))
}

// TestGetItemListError checks that getItem returns listing errors
// rather than fs.ErrorObjectNotFound.
func TestGetItemListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"access denied"}`, http.StatusForbidden)
	}))
	defer server.Close()
	f := testFs(t, server)

	_, err := f.getItem(context.Background(), "1000", "1", "file00000")
	require.Error(t, err)
	assert.NotErrorIs(t, err, fs.ErrorObjectNotFound)
	assert.Contains(t, err.Error(), "access denied")
}

// metaServer is a mock Drime server for the modtime and hash tests. It
// records the requests made as "METHOD path".
func metaServer(t *testing.T, verify func(w http.ResponseWriter), calls *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/model"):
			_, _ = w.Write([]byte(`{"fileEntry":{"id":7,"client_last_modified":1700000000123}}`))
		case strings.HasSuffix(r.URL.Path, "/metadata"):
			var req api.SetMetadataRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, int64(1700000000123), req.LastModified)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/verify-integrity"):
			verify(w)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

// TestModTime checks ModTime is read lazily, once, from the model
// endpoint and SetModTime sends milliseconds.
func TestModTime(t *testing.T) {
	var calls []string
	server := metaServer(t, nil, &calls)
	defer server.Close()
	o := &Object{fs: testFs(t, server), id: "7", modTime: time.Unix(1, 0)}

	want := time.UnixMilli(1700000000123)
	assert.True(t, want.Equal(o.ModTime(context.Background())))
	assert.True(t, want.Equal(o.ModTime(context.Background())))
	assert.Equal(t, []string{"GET /file-entries/7/model"}, calls)

	calls = nil
	require.NoError(t, o.SetModTime(context.Background(), want.Add(time.Nanosecond)))
	assert.Equal(t, []string{"POST /file-entries/7/metadata"}, calls)
	assert.True(t, want.Equal(o.modTime))
}

// TestFinishUpload checks the post upload hash verification
func TestFinishUpload(t *testing.T) {
	const sum = "82cf91c6f507ac8eb092f9fa036bafa3fe04478716707357c768d6927652b113"
	src := object.NewStaticObjectInfo("f", time.Now(), 1, true, map[hash.Type]string{hash.SHA256: sum}, nil)
	ctx := context.Background()
	mtime := int64(5)
	info := func() *api.Item {
		return &api.Item{ID: "7", ClientLastModified: &mtime}
	}

	t.Run("Verified", func(t *testing.T) {
		var calls []string
		server := metaServer(t, func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"verified":true,"serverHash":"` + sum + `"}`))
		}, &calls)
		defer server.Close()
		o := &Object{fs: testFs(t, server)}
		require.NoError(t, o.finishUpload(ctx, info(), src))
		h, err := o.Hash(ctx, hash.SHA256)
		require.NoError(t, err)
		assert.Equal(t, sum, h)
		assert.Equal(t, []string{"POST /file-entries/7/verify-integrity"}, calls)
	})

	t.Run("MismatchRemovesObject", func(t *testing.T) {
		var calls []string
		server := metaServer(t, func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"verified":false,"serverHash":"bad"}`))
		}, &calls)
		defer server.Close()
		o := &Object{fs: testFs(t, server)}
		err := o.finishUpload(ctx, info(), src)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "corrupted on transfer")
		require.Len(t, calls, 2)
		assert.Equal(t, "POST /file-entries/delete", calls[1])
		assert.Empty(t, o.sha256)
	})

	t.Run("ForbiddenIsIgnored", func(t *testing.T) {
		var calls []string
		server := metaServer(t, func(w http.ResponseWriter) {
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
		}, &calls)
		defer server.Close()
		o := &Object{fs: testFs(t, server)}
		require.NoError(t, o.finishUpload(ctx, info(), src))
		assert.Empty(t, o.sha256)
	})

	t.Run("NoSourceHash", func(t *testing.T) {
		var calls []string
		server := metaServer(t, nil, &calls)
		defer server.Close()
		o := &Object{fs: testFs(t, server)}
		nohash := object.NewStaticObjectInfo("f", time.Now(), 1, true, nil, nil)
		require.NoError(t, o.finishUpload(ctx, info(), nohash))
		assert.Empty(t, calls)
	})
}
