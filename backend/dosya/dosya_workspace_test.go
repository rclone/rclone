package dosya

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorkspaceServer answers GET /api/workspaces with the given body
func fakeWorkspaceServer(t *testing.T, body string, calls *int) rtFunc {
	return func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/workspaces", r.URL.Path)
		*calls++
		return jsonResp(http.StatusOK, body), nil
	}
}

func TestFindWorkspaceAdoptsTheOnlyOne(t *testing.T) {
	calls := 0
	f := newFakeFs(fakeWorkspaceServer(t, `{"ok":true,"workspaces":[{"id":"ws_only","name":"Mine"}]}`, &calls))
	f.opt.WorkspaceID = ""

	require.NoError(t, f.findWorkspace(context.Background()))
	assert.Equal(t, "ws_only", f.opt.WorkspaceID)
	assert.Equal(t, 1, calls)
}

func TestFindWorkspaceListsTheChoices(t *testing.T) {
	calls := 0
	f := newFakeFs(fakeWorkspaceServer(t, `{"ok":true,"workspaces":[`+
		`{"id":"ws_one","name":"One"},{"id":"ws_two","name":"Two"}]}`, &calls))
	f.opt.WorkspaceID = ""

	err := f.findWorkspace(context.Background())
	require.Error(t, err)
	// the ids have to be in the message: they are the only place the user
	// can read them, which is the whole point of listing them
	assert.Contains(t, err.Error(), "ws_one (One)")
	assert.Contains(t, err.Error(), "ws_two (Two)")
	assert.Empty(t, f.opt.WorkspaceID)
}

func TestFindWorkspaceWithNone(t *testing.T) {
	calls := 0
	f := newFakeFs(fakeWorkspaceServer(t, `{"ok":true,"workspaces":[]}`, &calls))
	f.opt.WorkspaceID = ""

	require.ErrorContains(t, f.findWorkspace(context.Background()), "no workspaces found")
}
