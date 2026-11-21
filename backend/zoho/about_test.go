package zoho

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aboutServer serves the JSON in responses for each API path and 404 for the rest.
func aboutServer(t *testing.T, responses map[string]string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func teamInfoJSON(used, allowed int64) string {
	return `{"data":{"attributes":{"storage_info":{"storage_used_in_bytes":` + strconv.FormatInt(used, 10) +
		`},"license_obj":{"allowed_space_bytes":` + strconv.FormatInt(allowed, 10) + `}}}}`
}

func TestAbout(t *testing.T) {
	const rootJSON = `{"data":{"id":"rootid","attributes":{"name":"root","is_folder":true,
		"storage_info":{"size_in_bytes":0,"storage_used_in_bytes":270755958,"files_count":1,"folders_count":0}}}}`
	const userJSON = `{"data":{"id":"zuid1"}}`
	team := func(id string) string { return `{"id":"` + id + `"}` }
	workspace := func(id string) string { return `{"id":"` + id + `"}` }
	list := func(items ...string) string {
		out := `{"data":[`
		for i, item := range items {
			if i > 0 {
				out += ","
			}
			out += item
		}
		return out + `]}`
	}

	for _, test := range []struct {
		name      string
		responses map[string]string
		wantUsed  int64
		wantTotal *int64
		wantFree  *int64
	}{
		{
			name: "team usage and quota",
			responses: map[string]string{
				"/files/rootid":        rootJSON,
				"/users/me":            userJSON,
				"/users/zuid1/teams":   list(team("t1"), team("t2")),
				"/teams/t1/workspaces": list(workspace("other")),
				"/teams/t2/workspaces": list(workspace("rootid")),
				"/teams/t2":            teamInfoJSON(1000, 5000),
				"/teams/t1":            teamInfoJSON(1, 2),
			},
			wantUsed:  1000,
			wantTotal: ptr(5000),
			wantFree:  ptr(4000),
		},
		{
			name: "unknown quota leaves total and free unset",
			responses: map[string]string{
				"/files/rootid":        rootJSON,
				"/users/me":            userJSON,
				"/users/zuid1/teams":   list(team("t1")),
				"/teams/t1/workspaces": list(workspace("rootid")),
				"/teams/t1":            teamInfoJSON(988506874, 0),
			},
			wantUsed: 988506874,
		},
		{
			name: "free never goes negative",
			responses: map[string]string{
				"/files/rootid":        rootJSON,
				"/users/me":            userJSON,
				"/users/zuid1/teams":   list(team("t1")),
				"/teams/t1/workspaces": list(workspace("rootid")),
				"/teams/t1":            teamInfoJSON(6000, 5000),
			},
			wantUsed:  6000,
			wantTotal: ptr(5000),
			wantFree:  ptr(0),
		},
		{
			name: "only team is used when root is not a workspace",
			responses: map[string]string{
				"/files/rootid":        rootJSON,
				"/users/me":            userJSON,
				"/users/zuid1/teams":   list(team("t1")),
				"/teams/t1/workspaces": list(workspace("other")),
				"/teams/t1":            teamInfoJSON(1000, 5000),
			},
			wantUsed:  1000,
			wantTotal: ptr(5000),
			wantFree:  ptr(4000),
		},
		{
			name: "ambiguous team falls back to root folder usage",
			responses: map[string]string{
				"/files/rootid":        rootJSON,
				"/users/me":            userJSON,
				"/users/zuid1/teams":   list(team("t1"), team("t2")),
				"/teams/t1/workspaces": list(workspace("a")),
				"/teams/t2/workspaces": list(workspace("b")),
			},
			wantUsed: 270755958,
		},
		{
			name: "failed team lookup falls back to root folder usage",
			responses: map[string]string{
				"/files/rootid": rootJSON,
			},
			wantUsed: 270755958,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ts := aboutServer(t, test.responses)
			f := &Fs{
				opt:      Options{RootFolderID: "rootid"},
				srv:      rest.NewClient(http.DefaultClient).SetRoot(ts.URL),
				pacer:    fs.NewPacer(context.Background(), pacer.NewGoogleDrive(pacer.MinSleep(0))),
				throttle: &throttleState{},
			}
			usage, err := f.About(context.Background())
			require.NoError(t, err)
			require.NotNil(t, usage.Used)
			assert.Equal(t, test.wantUsed, *usage.Used)
			assertUsageValue(t, test.wantTotal, usage.Total)
			assertUsageValue(t, test.wantFree, usage.Free)
		})
	}
}

func ptr(n int64) *int64 { return &n }

func assertUsageValue(t *testing.T, want, got *int64) {
	if want == nil {
		assert.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	assert.Equal(t, *want, *got)
}
