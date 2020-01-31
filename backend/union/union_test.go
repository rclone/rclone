// Test Union filesystem interface
package union_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/require"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   *fstest.RemoteName,
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir1 := filepath.Join(os.TempDir(), "rclone-union-test-standard1")
	tempdir2 := filepath.Join(os.TempDir(), "rclone-union-test-standard2")
	tempdir3 := filepath.Join(os.TempDir(), "rclone-union-test-standard3")
	require.NoError(t, os.MkdirAll(tempdir1, 0744))
	require.NoError(t, os.MkdirAll(tempdir2, 0744))
	require.NoError(t, os.MkdirAll(tempdir3, 0744))
	upstreams := tempdir1 + " " + tempdir2 + " " + tempdir3
	name := "TestUnion"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "union"},
			{Name: name, Key: "upstreams", Value: upstreams},
			{Name: name, Key: "action_policy", Value: "epall"},
			{Name: name, Key: "create_policy", Value: "epmfs"},
			{Name: name, Key: "search_policy", Value: "ff"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

func TestRO(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir1 := filepath.Join(os.TempDir(), "rclone-union-test-ro1")
	tempdir2 := filepath.Join(os.TempDir(), "rclone-union-test-ro2")
	tempdir3 := filepath.Join(os.TempDir(), "rclone-union-test-ro3")
	require.NoError(t, os.MkdirAll(tempdir1, 0744))
	require.NoError(t, os.MkdirAll(tempdir2, 0744))
	require.NoError(t, os.MkdirAll(tempdir3, 0744))
	upstreams := tempdir1 + " " + tempdir2 + ":ro " + tempdir3 + ":ro"
	name := "TestUnionRO"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "union"},
			{Name: name, Key: "upstreams", Value: upstreams},
			{Name: name, Key: "action_policy", Value: "epall"},
			{Name: name, Key: "create_policy", Value: "epmfs"},
			{Name: name, Key: "search_policy", Value: "ff"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

func TestNC(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir1 := filepath.Join(os.TempDir(), "rclone-union-test-nc1")
	tempdir2 := filepath.Join(os.TempDir(), "rclone-union-test-nc2")
	tempdir3 := filepath.Join(os.TempDir(), "rclone-union-test-nc3")
	require.NoError(t, os.MkdirAll(tempdir1, 0744))
	require.NoError(t, os.MkdirAll(tempdir2, 0744))
	require.NoError(t, os.MkdirAll(tempdir3, 0744))
	upstreams := tempdir1 + " " + tempdir2 + ":nc " + tempdir3 + ":nc"
	name := "TestUnionNC"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "union"},
			{Name: name, Key: "upstreams", Value: upstreams},
			{Name: name, Key: "action_policy", Value: "epall"},
			{Name: name, Key: "create_policy", Value: "epmfs"},
			{Name: name, Key: "search_policy", Value: "ff"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

func TestPolicy1(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir1 := filepath.Join(os.TempDir(), "rclone-union-test-policy11")
	tempdir2 := filepath.Join(os.TempDir(), "rclone-union-test-policy12")
	tempdir3 := filepath.Join(os.TempDir(), "rclone-union-test-policy13")
	require.NoError(t, os.MkdirAll(tempdir1, 0744))
	require.NoError(t, os.MkdirAll(tempdir2, 0744))
	require.NoError(t, os.MkdirAll(tempdir3, 0744))
	upstreams := tempdir1 + " " + tempdir2 + " " + tempdir3
	name := "TestUnionPolicy1"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "union"},
			{Name: name, Key: "upstreams", Value: upstreams},
			{Name: name, Key: "action_policy", Value: "all"},
			{Name: name, Key: "create_policy", Value: "lus"},
			{Name: name, Key: "search_policy", Value: "all"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

func TestPolicy2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir1 := filepath.Join(os.TempDir(), "rclone-union-test-policy21")
	tempdir2 := filepath.Join(os.TempDir(), "rclone-union-test-policy22")
	tempdir3 := filepath.Join(os.TempDir(), "rclone-union-test-policy23")
	require.NoError(t, os.MkdirAll(tempdir1, 0744))
	require.NoError(t, os.MkdirAll(tempdir2, 0744))
	require.NoError(t, os.MkdirAll(tempdir3, 0744))
	upstreams := tempdir1 + " " + tempdir2 + " " + tempdir3
	name := "TestUnionPolicy2"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "union"},
			{Name: name, Key: "upstreams", Value: upstreams},
			{Name: name, Key: "action_policy", Value: "all"},
			{Name: name, Key: "create_policy", Value: "rand"},
			{Name: name, Key: "search_policy", Value: "ff"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt", "DuplicateFiles"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}
