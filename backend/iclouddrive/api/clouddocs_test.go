package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocumentRecordNames checks that a bare file UUID maps to the upper-cased
// documentContent/documentStructure record names CloudDocs uses for both halves of
// a document.
func TestDocumentRecordNames(t *testing.T) {
	content, structure := documentRecordNames("abc-123")
	assert.Equal(t, "documentContent/ABC-123", content)
	assert.Equal(t, "documentStructure/ABC-123", structure)

	// Already upper-case input is left unchanged.
	content, structure = documentRecordNames("DEF-456")
	assert.Equal(t, "documentContent/DEF-456", content)
	assert.Equal(t, "documentStructure/DEF-456", structure)
}

// TestDirectoryRecordName checks that a bare folder UUID maps to the upper-cased
// directory record name CloudDocs uses for shared-folder child directories.
func TestDirectoryRecordName(t *testing.T) {
	assert.Equal(t, "directory/ABC-123", directoryRecordName("abc-123"))
	assert.Equal(t, "directory/DEF-456", directoryRecordName("DEF-456"))
}

// TestDeleteRecordOperations checks the records/modify delete operations: one per
// record, in content-then-structure order, each carrying its current change tag (an
// unknown tag is sent empty).
func TestDeleteRecordOperations(t *testing.T) {
	content, structure := documentRecordNames("abc-123")
	tags := map[string]string{
		content:   "tag-content",
		structure: "tag-structure",
	}
	ops := deleteRecordOperations(content, structure, tags)

	require.Len(t, ops, 2)
	assert.Equal(t, "delete", ops[0].OperationType)
	assert.Equal(t, content, ops[0].Record.RecordName)
	assert.Equal(t, "tag-content", ops[0].Record.RecordChangeTag)
	assert.Equal(t, "delete", ops[1].OperationType)
	assert.Equal(t, structure, ops[1].Record.RecordName)
	assert.Equal(t, "tag-structure", ops[1].Record.RecordChangeTag)
}

// TestDeleteRecordOperationsMissingTag verifies a record with no looked-up change
// tag gets an empty tag rather than panicking on the map lookup.
func TestDeleteRecordOperationsMissingTag(t *testing.T) {
	content, structure := documentRecordNames("abc-123")
	ops := deleteRecordOperations(content, structure, map[string]string{content: "only-content"})

	require.Len(t, ops, 2)
	assert.Equal(t, "only-content", ops[0].Record.RecordChangeTag)
	assert.Equal(t, "", ops[1].Record.RecordChangeTag)
}

// TestDeleteDirectoryRecordOperation checks the single records/modify delete
// operation used for rmdir of an empty shared sub-directory.
func TestDeleteDirectoryRecordOperation(t *testing.T) {
	dir := directoryRecordName("abc-123")
	ops := deleteDirectoryRecordOperation(dir, map[string]string{dir: "tag-dir"})

	require.Len(t, ops, 1)
	assert.Equal(t, "delete", ops[0].OperationType)
	assert.Equal(t, dir, ops[0].Record.RecordName)
	assert.Equal(t, "tag-dir", ops[0].Record.RecordChangeTag)
}

// TestDeleteRequestJSON locks the wire format of a shared-zone delete: a
// non-atomic records/modify with two delete operations and the bare record names.
func TestDeleteRequestJSON(t *testing.T) {
	content, structure := documentRecordNames("abc-123")
	zone := &CKZoneID{ZoneName: "com.apple.CloudDocs", OwnerRecordName: "_owner", ZoneType: "REGULAR_CUSTOM_ZONE"}
	body := ckModifyRequest{
		Atomic:     false,
		ZoneID:     zone,
		Operations: deleteRecordOperations(content, structure, map[string]string{content: "t1", structure: "t2"}),
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, false, got["atomic"])
	ops, ok := got["operations"].([]any)
	require.True(t, ok)
	require.Len(t, ops, 2)

	op0 := ops[0].(map[string]any)
	assert.Equal(t, "delete", op0["operationType"])
	rec0 := op0["record"].(map[string]any)
	assert.Equal(t, "documentContent/ABC-123", rec0["recordName"])
	assert.Equal(t, "t1", rec0["recordChangeTag"])
	// A delete carries only the record name + change tag; no fields/parent are emitted.
	_, hasFields := rec0["fields"]
	assert.False(t, hasFields)
	_, hasParent := rec0["parent"]
	assert.False(t, hasParent)
}

// TestDeleteDirectoryRequestJSON locks the wire format for shared-directory
// rmdir: a non-atomic records/modify with one delete operation for directory/<uuid>.
func TestDeleteDirectoryRequestJSON(t *testing.T) {
	dir := directoryRecordName("abc-123")
	zone := &CKZoneID{ZoneName: "com.apple.CloudDocs", OwnerRecordName: "_owner", ZoneType: "REGULAR_CUSTOM_ZONE"}
	body := ckModifyRequest{
		Atomic:     false,
		ZoneID:     zone,
		Operations: deleteDirectoryRecordOperation(dir, map[string]string{dir: "td"}),
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, false, got["atomic"])
	ops, ok := got["operations"].([]any)
	require.True(t, ok)
	require.Len(t, ops, 1)

	op0 := ops[0].(map[string]any)
	assert.Equal(t, "delete", op0["operationType"])
	rec0 := op0["record"].(map[string]any)
	assert.Equal(t, "directory/ABC-123", rec0["recordName"])
	assert.Equal(t, "td", rec0["recordChangeTag"])
	_, hasFields := rec0["fields"]
	assert.False(t, hasFields)
	_, hasParent := rec0["parent"]
	assert.False(t, hasParent)
}

// TestLookupRecordTags extracts current change tags from lookup results before a
// delete request. It intentionally keeps unknown records out of the map.
func TestLookupRecordTags(t *testing.T) {
	tags := lookupRecordTags([]ckRecord{
		{RecordName: "directory/A", RecordChangeTag: "ta"},
		{RecordName: "documentStructure/B", RecordChangeTag: "tb"},
	})
	assert.Equal(t, map[string]string{"directory/A": "ta", "documentStructure/B": "tb"}, tags)
}

// TestCheckRecordErrors documents how CloudKit per-record failures are surfaced
// from records/modify responses.
func TestCheckRecordErrors(t *testing.T) {
	require.NoError(t, checkRecordErrors("op", []ckRecord{{RecordName: "directory/A"}}))

	err := checkRecordErrors("op", []ckRecord{{
		RecordName:      "directory/A",
		Reason:          "not allowed",
		ServerErrorCode: "ACCESS_DENIED",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op failed for directory/A: not allowed (ACCESS_DENIED)")
}

// TestDocumentBaseHash checks the basehash used to locate a document by name inside
// a shared folder: base64(SHA256(basename)) with the extension stripped.
func TestDocumentBaseHash(t *testing.T) {
	// base64(sha256("report")), computed independently.
	const reportHash = "hF6RgxMZ6JxNZWvbgMJ4rAmnIw1h5d/S4bH7tDasiRc="
	assert.Equal(t, reportHash, documentBaseHash("report"))
	// The (single) extension is stripped before hashing, so "report" and
	// "report.<ext>" collide.
	assert.Equal(t, reportHash, documentBaseHash("report.txt"))
	assert.Equal(t, reportHash, documentBaseHash("report.pdf"))
	assert.Equal(t, reportHash, documentBaseHash("report.tar"))

	// Only the FINAL extension is stripped (matches strings.LastIndexByte), so
	// "report.tar.gz" hashes "report.tar", not "report".
	assert.NotEqual(t, reportHash, documentBaseHash("report.tar.gz"))
}

// TestDocumentBaseHashEdgeCases documents the dot-handling: a leading dot is part of
// the name (LastIndexByte must be > 0 to strip), and a name with no dot is hashed
// whole.
func TestDocumentBaseHashEdgeCases(t *testing.T) {
	// base64(sha256("noext")), computed independently.
	const noextHash = "yEfweOKyUPB5TsC6TUTlFvSAkkD+MQJM3r9XArkpbTk="
	// No dot: the name is hashed whole.
	assert.Equal(t, noextHash, documentBaseHash("noext"))
	// Stripping the trailing extension makes "noext.txt" collide with "noext".
	assert.Equal(t, noextHash, documentBaseHash("noext.txt"))
	// A leading dot is NOT stripped (index 0 is not > 0), so a dotfile keeps its
	// full name and differs from the same text without the leading dot.
	assert.NotEqual(t, documentBaseHash(".hidden"), documentBaseHash("hidden"))
}
