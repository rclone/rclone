package tarkle

import (
	"context"
	"testing"

	"github.com/rclone/rclone/backend/tarkle/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tarkle compares names by NFC, one space for each run of white space, trim
// and lower case (normalizeName in lib/file-version-core.mjs). The backend
// has to find "the file that blocks this name" the same way, or a replace
// leaves the old file in place and the rename fails with name_taken.
func TestNameKeyIsTheKeyTarkleUses(t *testing.T) {
	for _, pair := range [][2]string{
		{"MEDIUM.bin", "medium.bin"},
		{"Café.txt", "Café.txt"}, // precomposed é and e + combining accent (macOS writes the second)
		{"a  b.txt", "a b.txt"},
		{"a\tb.txt", "a b.txt"},
		{" a.txt ", "a.txt"},
		{"a b.txt", "a b.txt"}, // no-break space
	} {
		assert.Equal(t, nameKey(pair[0]), nameKey(pair[1]), "%q and %q are the same name to Tarkle", pair[0], pair[1])
	}
	for _, pair := range [][2]string{
		{"a.txt", "b.txt"},
		{"a b.txt", "ab.txt"},
		{"a.txt", "a.txt.bak"},
	} {
		assert.NotEqual(t, nameKey(pair[0]), nameKey(pair[1]), "%q and %q are different names", pair[0], pair[1])
	}
}

func TestReplaceExistingFindsTheSameNameInAnotherForm(t *testing.T) {
	// the old file is stored decomposed and with a doubled space; the new one is asked to take the composed, single-space name
	f, s := newScriptServer(t, nil, replaceAnswers(fileJSON("old", "Café  menu.txt")+","+fileJSON("new", "Café menu (2).txt"), nil))
	item := &api.Item{ID: "new", Type: api.ItemTypeFile, Name: "Café menu (2).txt"}
	require.NoError(t, f.replaceExisting(context.Background(), "", "Café menu.txt", item))
	assert.Equal(t, []string{"PATCH /files/old", "PATCH /files/new", "DELETE /files/old"}, s.writes(), "the old file is found, set aside and binned after the swap")
}

func folderJSON(id, name string) string {
	return `{"id":"` + id + `","type":"folder","name":"` + name + `","size":0,"parent_id":null,"modified_at":"2020-01-02T03:04:05.000Z"}`
}

// The database only refuses two siblings with the same lower(name), so "a b" and "a  b" are two folders. Walking a path must pick the one asked for.
func TestFindLeafUsesTheDatabasesRuleOnly(t *testing.T) {
	f, _ := newScriptServer(t, nil, replaceAnswers(folderJSON("f1", "a b")+","+folderJSON("f2", "a  b"), nil))
	for name, want := range map[string]string{"a b": "f1", "a  b": "f2", "A B": "f1", "A  B": "f2"} {
		id, found, err := f.FindLeaf(context.Background(), "", name)
		require.NoError(t, err, name)
		require.True(t, found, name)
		assert.Equal(t, want, id, name)
	}
	_, found, err := f.FindLeaf(context.Background(), "", "a   b")
	require.NoError(t, err)
	assert.False(t, found, "a name that differs in spacing is not the folder asked for")
}

// Two files that differ only in spacing can both exist: the one to replace is the one of exactly the asked name.
func TestReplaceExistingPrefersTheExactNameOverALooserOne(t *testing.T) {
	f, s := newScriptServer(t, nil, replaceAnswers(fileJSON("loose", "a  b.txt")+","+fileJSON("exact", "a b.txt")+","+fileJSON("new", "a b (2).txt"), nil))
	item := &api.Item{ID: "new", Type: api.ItemTypeFile, Name: "a b (2).txt"}
	require.NoError(t, f.replaceExisting(context.Background(), "", "a b.txt", item))
	assert.Equal(t, []string{"PATCH /files/exact", "PATCH /files/new", "DELETE /files/exact"}, s.writes(), "the file of exactly this name is replaced, the other is left alone")
}
