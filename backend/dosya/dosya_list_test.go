package dosya

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeListServer serves GET /api/files the way the API does: files paged at
// per_page, every folder repeated on each page
func fakeListServer(t *testing.T, totalFiles int, pages *[]int) rtFunc {
	return func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/files", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "name", q.Get("sort"))
		assert.Equal(t, "asc", q.Get("dir"))
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		require.Positive(t, page)
		require.Positive(t, perPage)
		*pages = append(*pages, page)

		var files []string
		for i := (page - 1) * perPage; i < page*perPage && i < totalFiles; i++ {
			files = append(files, fmt.Sprintf(`{"id":"fil_%d","name":"f%05d.txt","size":1}`, i, i))
		}
		totalPages := max(1, (totalFiles+perPage-1)/perPage)
		body := fmt.Sprintf(`{"ok":true,"files":[%s],"folders":[{"id":"fld_1","name":"sub"}],`+
			`"pagination":{"page":%d,"per_page":%d,"total_files":%d,"total_pages":%d}}`,
			strings.Join(files, ","), page, perPage, totalFiles, totalPages)
		return jsonResp(http.StatusOK, body), nil
	}
}

func TestListFilesAndFoldersFollowsPages(t *testing.T) {
	var pages []int
	f := newFakeFs(fakeListServer(t, 2*listPageSize+7, &pages))

	listing, err := f.listFilesAndFolders(context.Background(), "fld_parent")
	require.NoError(t, err)

	assert.Equal(t, []int{1, 2, 3}, pages)
	require.Len(t, listing.Files, 2*listPageSize+7)
	assert.Equal(t, "f01006.txt", listing.Files[len(listing.Files)-1].Name)
	// folders repeat on every page and must not be duplicated
	assert.Len(t, listing.Folders, 1)
}

func TestListFilesAndFoldersSinglePage(t *testing.T) {
	for _, total := range []int{0, 1, listPageSize} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			var pages []int
			f := newFakeFs(fakeListServer(t, total, &pages))

			listing, err := f.listFilesAndFolders(context.Background(), rootID)
			require.NoError(t, err)

			assert.Equal(t, []int{1}, pages)
			assert.Len(t, listing.Files, total)
		})
	}
}
