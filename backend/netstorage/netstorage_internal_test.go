package netstorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListResumeCannotChangeHost(t *testing.T) {
	// The resume start returned by a list request comes from the
	// server. It must not be able to move the signed continuation
	// request to another host.
	var requests int
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer other.Close()
	otherHost := strings.TrimPrefix(other.URL, "http://")

	var listRequests int
	configured := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("X-Akamai-ACS-Action")
		switch {
		case strings.Contains(action, "action=stat"):
			_, _ = w.Write([]byte(`<stat directory="/123456/dir"><file type="dir" name="dir" mtime="0"/></stat>`))
		case strings.Contains(action, "action=list"):
			listRequests++
			_, _ = w.Write([]byte(`<list><file type="file" name="123456/dir/a.txt" size="1" mtime="0"/><resume start="//` + otherHost + `/123456/dir/b.txt"/></list>`))
		default:
			http.Error(w, "unexpected action "+action, http.StatusBadRequest)
		}
	}))
	defer configured.Close()

	m := configmap.Simple{
		"type":     "netstorage",
		"protocol": "http",
		"host":     strings.TrimPrefix(configured.URL, "http://") + "/123456/",
		"account":  "account",
		"secret":   obscure.MustObscure("secret"),
	}
	f, err := NewFs(context.Background(), "TestNetStorage", "dir", m)
	require.NoError(t, err)

	// The listing must fail loudly rather than with
	// fs.ErrorDirNotFound which sync treats as an empty directory
	err = f.(*Fs).ListR(context.Background(), "", func(entries fs.DirEntries) error { return nil })
	assert.ErrorContains(t, err, "must not change the scheme, host or user")
	assert.NotErrorIs(t, err, fs.ErrorDirNotFound)
	assert.Equal(t, 1, listRequests, "list request sent to the configured host")
	assert.Equal(t, 0, requests, "request sent to the other host")
}
