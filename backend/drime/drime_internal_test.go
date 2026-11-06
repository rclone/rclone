package drime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/drime/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/dircache"
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

// treeServer serves /drive/file-entries from tree, the children of each
// folder keyed by its ID, the way Drime does: testPerPage entries a page
// with a total and last_page which only say whether there is another
// page. A parentIds[] listing including a folder in refuse answers 500.
//
// It records the parentIds[] of each first page asked for in groups.
func treeServer(t *testing.T, tree map[string][]api.Item, refuse map[string]bool, groups *[][]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		page, err := strconv.Atoi(params.Get("page"))
		require.NoError(t, err)
		parents := params["parentIds[]"]
		if page == 1 && len(parents) > 0 {
			*groups = append(*groups, parents)
		}
		if id := params.Get("folderId"); id != "" {
			parents = []string{id}
		} else {
			for _, id := range parents {
				if refuse[id] {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
		var entries []api.Item
		for _, id := range parents {
			entries = append(entries, tree[id]...)
		}
		start := min((page-1)*testPerPage, len(entries))
		end := min(start+testPerPage, len(entries))
		lastPage := page
		if end < len(entries) {
			lastPage = page + 1
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"current_page": page,
			"last_page":    lastPage,
			"per_page":     testPerPage,
			"total":        (page + 1) * testPerPage,
			"data":         entries[start:end],
		}))
	}))
}

// TestListR checks that ListR lists every entry of a tree exactly once,
// asking for the children of up to listRGrouping folders at a time, and
// lists folders drime won't list together one at a time.
func TestListR(t *testing.T) {
	// 60 folders under the root, each holding 3 files and a folder
	// holding 2 files
	tree := map[string][]api.Item{}
	subIDs := map[string]string{}
	var want []string
	id := 10000
	add := func(parentID, name, itemType string) string {
		id++
		itemID := strconv.Itoa(id)
		tree[parentID] = append(tree[parentID], api.Item{ID: json.Number(itemID), Name: name, Type: itemType, ParentID: json.Number(parentID)})
		return itemID
	}
	for i := range 60 {
		dir := fmt.Sprintf("dir%02d", i)
		dirID := add("1", dir, api.ItemTypeFolder)
		want = append(want, dir, dir+"/sub")
		for j := range 3 {
			add(dirID, fmt.Sprintf("file%d", j), "text")
			want = append(want, fmt.Sprintf("%s/file%d", dir, j))
		}
		subID := add(dirID, "sub", api.ItemTypeFolder)
		subIDs[dir] = subID
		for j := range 2 {
			add(subID, fmt.Sprintf("file%d", j), "text")
			want = append(want, fmt.Sprintf("%s/sub/file%d", dir, j))
		}
	}
	refusedID := tree["1"][7].ID.String()

	for _, test := range []struct {
		name   string
		refuse map[string]bool
	}{
		{name: "Grouped"},
		{name: "Refused", refuse: map[string]bool{refusedID: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var groups [][]string
			server := treeServer(t, tree, test.refuse, &groups)
			defer server.Close()
			f := testFs(t, server)
			f.dirCache = dircache.New("", "1", f)

			var got []string
			require.NoError(t, f.ListR(ctx, "", func(entries fs.DirEntries) error {
				for _, entry := range entries {
					got = append(got, entry.Remote())
				}
				return nil
			}))
			assert.ElementsMatch(t, want, got)

			var groupSizes []int
			for _, group := range groups {
				groupSizes = append(groupSizes, len(group))
			}
			// two levels of 60 folders, each group asked for once
			assert.Equal(t, []int{50, 10, 50, 10}, groupSizes)

			dirID, err := f.dirCache.FindDir(ctx, "dir59/sub", false)
			require.NoError(t, err)
			assert.Equal(t, subIDs["dir59"], dirID, "directory IDs cached")
		})
	}
}
