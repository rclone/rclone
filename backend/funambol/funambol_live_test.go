package funambol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/funambol/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

// TestLiveProbe checks upload restrictions and overwrites in an isolated folder.
// Set RCLONE_FUNAMBOL_PROBE_REMOTE and RCLONE_CONFIG to run it against an account.
// Its files are removed afterwards, leaving deleted items in the account trash.
func TestLiveProbe(t *testing.T) {
	remote := os.Getenv("RCLONE_FUNAMBOL_PROBE_REMOTE")
	if remote == "" {
		t.Skip("set RCLONE_FUNAMBOL_PROBE_REMOTE for the isolated live probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx, ci := fs.AddConfig(ctx)
	ci.AskPassword = false
	ci.LowLevelRetries = 1
	require.NoError(t, config.SetConfigPath(os.Getenv("RCLONE_CONFIG")))
	configfile.Install()
	base, err := fs.NewFs(ctx, remote)
	require.NoError(t, err)
	f := base.(*Fs)
	require.Empty(t, f.root, "probe requires an account-root remote")
	t.Cleanup(f.client.CloseIdleConnections)

	name := fmt.Sprintf("rclone-probe-%d", time.Now().UnixNano())
	dirID, err := f.CreateDir(ctx, f.rootID, name)
	require.NoError(t, err)
	f.dirCache.Put(name, dirID)
	created := make(map[string]bool)
	t.Cleanup(func() {
		clean, done := context.WithTimeout(context.Background(), 2*time.Minute)
		defer done()
		if err := f.listMedia(clean, dirID, func(item *api.Item) { created[item.ID.String()] = true }); err != nil {
			t.Errorf("list probe files for cleanup: %v", err)
		}
		var ids []string
		for id := range created {
			if id != "" {
				ids = append(ids, id)
			}
		}
		if err := f.deleteMedia(clean, ids...); err != nil {
			t.Errorf("delete probe files: %v", err)
			return
		}
		if err := f.deleteFolder(clean, dirID); err != nil {
			t.Errorf("delete probe folder: %v", err)
		}
		t.Logf("cleanup: removed %d item(s) and the probe folder", len(ids))
	})
	put := func(leaf string, data []byte) (*Object, error) {
		src := object.NewStaticObjectInfo(name+"/"+leaf, time.Now(), int64(len(data)), true, nil, f)
		o, err := f.Put(ctx, bytes.NewReader(data), src)
		if o != nil && o.(*Object).id != "" {
			created[o.(*Object).id] = true
		}
		if err != nil {
			return nil, err
		}
		return o.(*Object), nil
	}
	verify := func(o *Object, want []byte) {
		t.Helper()
		item, err := f.waitForFile(ctx, o.id, "", dirID, int64(len(want)))
		require.NoError(t, err)
		o.url = item.URL
		rc, err := o.Open(ctx)
		require.NoError(t, err)
		got, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	for _, leaf := range []string{"refused.tmp", "refused.TMP"} {
		_, err := put(leaf, []byte("refused"))
		require.Error(t, err)
		require.True(t, fserrors.IsNoRetryError(err), err)
		t.Logf("%s refused without retry", leaf)
	}
	for _, leaf := range []string{"allowed.tmp.bin", "allowed.tmpx"} {
		_, err := put(leaf, []byte("allowed"))
		require.NoError(t, err)
	}

	v1 := bytes.Repeat([]byte{1}, 5000)
	o, err := put("overwrite.bin", v1)
	require.NoError(t, err)
	id := o.id
	verify(o, v1)
	v2 := bytes.Repeat([]byte{2}, 7000)
	src := object.NewStaticObjectInfo(o.remote, time.Now(), int64(len(v2)), true, nil, f)
	require.NoError(t, o.Update(ctx, bytes.NewReader(v2), src))
	require.Equal(t, id, o.id, "Update preserves the item ID")
	verify(o, v2)
	t.Logf("Update preserved the item ID and downloaded %d matching bytes", len(v2))
	v3 := bytes.Repeat([]byte{3}, 9000)
	o, err = put("overwrite.bin", v3)
	require.NoError(t, err)
	require.Equal(t, id, o.id, "Put reuses the existing item")
	verify(o, v3)
	t.Logf("Put preserved the item ID and downloaded %d matching bytes", len(v3))
	for {
		var matches []api.Item
		require.NoError(t, f.listMedia(ctx, dirID, func(item *api.Item) {
			if item.Name == "overwrite.bin" || item.Name == "overwrite (1).bin" {
				matches = append(matches, *item)
			}
		}))
		if len(matches) != 0 {
			require.Len(t, matches, 1)
			require.Equal(t, id, matches[0].ID.String())
			require.Equal(t, int64(len(v3)), matches[0].Size)
			break
		}
		require.NoError(t, waitFileOperation(ctx))
	}
}
